package dobermann

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/welthee/dobermann/key"
	"github.com/welthee/dobermann/nonce"
	"github.com/welthee/dobermann/transactor"
	"math/big"
	"os"
	"time"
)

const (
	nonceTooLow                       = "nonce too low"
	alreadyKnown                      = "already known"
	replacementTransactionUnderpriced = "replacement transaction underpriced"
	minLogLevel                       = zerolog.Disabled
)

var (
	StatusFail               Status            = "fail"
	StatusSuccess            Status            = "success"
	StatusPending            Status            = "pending"
	StatusSkip               Status            = "skip"
	NonceProviderTypeFixed   NonceProviderType = "fixed"
	NonceProviderTypeNetwork NonceProviderType = "network"
)

// Collector provides method to collect ERC-20 tokens in a specific account from other given accounts
type Collector interface {
	Collect(ctx context.Context, collectionAcount DestinationAccount, accounts []SourceAccount) []Result
	GetChainId(ctx context.Context) *big.Int
}

type Status string
type NonceProviderType string

// Result the outcome of the ERC-20 collection for a SourceAccount
type Result struct {
	Status        Status
	SourceAccount SourceAccount
}

// SourceAccount keeps the details of the account from which the tokens are collected
type SourceAccount struct {
	KeyProvider key.Provider
	Token       string
	Amount      string
}

// DestinationAccount which provides the gas for the collection and receives the ERC-20 tokens
type DestinationAccount struct {
	KeyProvider key.Provider
}

// EVMCollectorConfig contains network configuration
type EVMCollectorConfig struct {
	BlockchainUrl     string
	GasTrackerUrl     string
	NonceProviderType NonceProviderType
	LoggerKind        string
	LoggerLevel       string
}

// NewEVMCollector utility method to create a EVM collector
// using the provided EVMCollectorConfig
func NewEVMCollector(config EVMCollectorConfig) (Collector, error) {
	logLevel, err := zerolog.ParseLevel(config.LoggerLevel)
	if err != nil {
		logLevel = minLogLevel
	}
	switch config.LoggerKind {
	case "console":
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(logLevel)
	default:
		log.Debug().Msg("using json logger")
	}
	zerolog.DefaultContextLogger = &log.Logger

	client, err := ethclient.Dial(config.BlockchainUrl)
	if err != nil {
		return nil, err
	}
	gasTracker := transactor.NewPolygonGasTracker(config.GasTrackerUrl)

	var nonceProvider nonce.Provider
	switch config.NonceProviderType {
	case NonceProviderTypeNetwork:
		nonceProvider = nonce.NewNetworkNonceProvider(client)
	default:
		nonceProvider = nonce.NewFixedNonceProvider(nil)

	}

	chainId, err := client.ChainID(context.TODO())
	if err != nil {
		return nil, err
	}
	transactor, err := transactor.NewEvmTransactor(client, gasTracker, nonceProvider)
	if err != nil {
		return nil, err
	}

	return evmCollector{
		transactor: transactor,
		chainId:    chainId,
	}, nil
}

type evmCollector struct {
	transactor transactor.Transactor
	chainId    *big.Int
}

func (c evmCollector) GetChainId(ctx context.Context) *big.Int {
	return c.chainId
}

func (c evmCollector) Collect(ctx context.Context, destinationAccount DestinationAccount, accounts []SourceAccount) []Result {
	var results = make([]Result, 0)

	for _, account := range accounts {
		results = append(results, c.collect(ctx, account, destinationAccount))
	}

	return results
}

func (c evmCollector) getTokenBalance(ctx context.Context, toBeCollectedAccountAddr *common.Address, key SourceAccount) (*big.Int, error) {
	accountToBeCollectedERC20Balance, err := c.transactor.BalanceOf(ctx, *toBeCollectedAccountAddr, key.Token)
	if err != nil {
		return nil, err
	}

	return accountToBeCollectedERC20Balance, nil
}

func (c evmCollector) collect(ctx context.Context, account SourceAccount, destinationAccount DestinationAccount) Result {
	tokenBalance, err := c.getTokenBalance(ctx, account.KeyProvider.GetAddress(), account)
	if err != nil {
		return handleError(ctx, account, err)
	}

	if tokenBalance.Cmp(big.NewInt(0)) == 0 {
		return getResult(ctx, account, StatusSkip)
	}

	amount := account.Amount
	if amount != "" {
		a, _ := new(big.Int).SetString(amount, 10)
		if tokenBalance.Cmp(a) < 0 {
			return handleError(ctx, account, errors.New("insufficient balance"))
		}
	} else {
		amount = tokenBalance.String()
	}

	gasTipCapValue, gasFeeCapValue, err := c.transactor.GetGasCapValues(ctx)
	if err != nil {
		return handleError(ctx, account, err)
	}

	ecr20TxParams := transactor.TxParams{
		TokenAddr:           account.Token,
		SenderKeyProvider:   account.KeyProvider,
		ReceiverKeyProvider: destinationAccount.KeyProvider,
		Amount:              amount,
		GasTipCapValue:      gasTipCapValue,
		GasFeeCapValue:      gasFeeCapValue,
	}
	erc20Tx, err := c.transactor.CreateERC20Tx(ctx, ecr20TxParams)
	if err != nil {
		return handleError(ctx, account, err)
	}
	estimatedFee := new(big.Int).Add(new(big.Int).Mul(big.NewInt(int64(erc20Tx.Gas())), gasFeeCapValue), gasTipCapValue)
	accountToBeCollectedBalance, err := c.transactor.BalanceAt(ctx, *account.KeyProvider.GetAddress())
	if err != nil {
		return handleError(ctx, account, err)
	}

	remainingFee := new(big.Int).Sub(estimatedFee, accountToBeCollectedBalance)

	if remainingFee.Cmp(big.NewInt(0)) > 0 {
		nativTxParams := transactor.TxParams{
			SenderKeyProvider:   destinationAccount.KeyProvider,
			ReceiverKeyProvider: account.KeyProvider,
			Amount:              remainingFee.String(),
			GasTipCapValue:      gasTipCapValue,
			GasFeeCapValue:      gasFeeCapValue,
		}
		nativTx, err := c.transactor.CreateTx(ctx, nativTxParams)
		if err != nil {
			return handleError(ctx, account, err)
		}

		err = c.transactor.Transfer(ctx, nativTx)
		if err != nil {
			return handleError(ctx, account, err)
		}

		timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelFunc()
		isMined, err := c.transactor.VerifyTx(timeoutCtx, nativTx.Hash().Hex())
		if err != nil {
			return handleError(ctx, account, err)
		}

		if !isMined {
			return handleError(ctx, account, err)
		}

	}

	err = c.transactor.Transfer(ctx, erc20Tx)
	if err != nil {
		switch err.Error() {
		case nonceTooLow:
			return getResult(ctx, account, StatusSkip)
		case alreadyKnown:
			fallthrough
		case replacementTransactionUnderpriced:
			return getResult(ctx, account, StatusPending)
		default:
			return handleError(ctx, account, err)
		}
	}

	timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelFunc()
	isMined, err := c.transactor.VerifyTx(timeoutCtx, erc20Tx.Hash().Hex())
	if err != nil {
		return handleError(ctx, account, err)
	}
	if !isMined {
		return getResult(ctx, account, StatusPending)

	}
	return getResult(ctx, account, StatusSuccess)

}

func getResult(ctx context.Context, account SourceAccount, status Status) Result {
	result := Result{
		SourceAccount: account,
		Status:        status,
	}
	log.Ctx(ctx).Debug().
		Str("account", account.KeyProvider.GetAddress().Hex()).
		Str("status", string(status)).
		Msg("got result")
	return result
}

func handleError(ctx context.Context, account SourceAccount, err error) Result {
	log.Ctx(ctx).Debug().Err(err).
		Str("account", account.KeyProvider.GetAddress().Hex()).
		Msg("got error")
	return getResult(ctx, account, StatusFail)
}
