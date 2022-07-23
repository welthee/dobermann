package dobermann

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/welthee/dobermann/key"
	"github.com/welthee/dobermann/nonce"
	"github.com/welthee/dobermann/transactor"
	"math/big"
	"time"
)

const (
	nonceTooLow                       = "nonce too low"
	alreadyKnown                      = "already known"
	replacementTransactionUnderpriced = "replacement transaction underpriced"
)

var (
	StatusFail               Status            = "fail"
	StatusSuccess            Status            = "success"
	StatusPending            Status            = "pending"
	StatusSkip               Status            = "skip"
	NonceProviderTypeFixed   NonceProviderType = "fixed"
	NonceProviderTypeNetwork NonceProviderType = "network"
)

//Collector provides method to collect ERC-20 tokens in a specific account from other given accounts
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

//EVMCollectorConfig contains network configuration
type EVMCollectorConfig struct {
	BlockchainUrl     string
	GasTrackerUrl     string
	NonceProviderType NonceProviderType
}

// NewEVMCollector utility method to create a EVM collector
// using the provided EVMCollectorConfig
func NewEVMCollector(config EVMCollectorConfig) (Collector, error) {
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

		hasTokenToCollect, err := c.hasTokenToCollect(ctx, account.KeyProvider.GetAddress(), account)
		if err != nil {
			results = append(results, getResult(account, StatusFail))
			continue
		}

		if !hasTokenToCollect {
			results = append(results, getResult(account, StatusSkip))
			continue
		}

		gasTipCapValue, gasFeeCapValue, err := c.transactor.GetGasCapValues(ctx)
		if err != nil {
			results = append(results, getResult(account, StatusFail))
			continue
		}

		ecr20TxParams := transactor.TxParams{
			TokenAddr:           account.Token,
			SenderKeyProvider:   account.KeyProvider,
			ReceiverKeyProvider: destinationAccount.KeyProvider,
			Amount:              account.Amount,
			GasTipCapValue:      gasTipCapValue,
			GasFeeCapValue:      gasFeeCapValue,
		}
		erc20Tx, err := c.transactor.CreateERC20Tx(ctx, ecr20TxParams)
		if err != nil {
			results = append(results, getResult(account, StatusFail))
			continue
		}
		estimatedFee := new(big.Int).Add(new(big.Int).Mul(big.NewInt(int64(erc20Tx.Gas())), gasFeeCapValue), gasTipCapValue)
		accountToBeCollectedBalance, err := c.transactor.BalanceAt(ctx, *account.KeyProvider.GetAddress())
		if err != nil {
			results = append(results, getResult(account, StatusFail))
			continue
		}

		if accountToBeCollectedBalance.Cmp(big.NewInt(0)) > 0 {
			remainingFee := new(big.Int).Sub(estimatedFee, accountToBeCollectedBalance)

			if remainingFee.Cmp(big.NewInt(0)) >= 0 {
				nativTxParams := transactor.TxParams{
					SenderKeyProvider:   destinationAccount.KeyProvider,
					ReceiverKeyProvider: account.KeyProvider,
					Amount:              remainingFee.String(),
					GasTipCapValue:      gasTipCapValue,
					GasFeeCapValue:      gasFeeCapValue,
				}
				nativTx, err := c.transactor.CreateTx(ctx, nativTxParams)
				if err != nil {
					results = append(results, getResult(account, StatusFail))
					continue
				}

				err = c.transactor.Transfer(ctx, nativTx)
				if err != nil {
					results = append(results, getResult(account, StatusFail))
					continue
				}

				timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Minute)
				defer cancelFunc()
				isMined, err := c.transactor.VerifyTx(timeoutCtx, nativTx.Hash().Hex())
				if err != nil {
					results = append(results, getResult(account, StatusFail))
					continue
				}

				if !isMined {
					results = append(results, getResult(account, StatusFail))
					continue
				}
			}

		}

		err = c.transactor.Transfer(ctx, erc20Tx)
		if err != nil {
			switch err.Error() {
			case nonceTooLow:
				results = append(results, getResult(account, StatusSkip))
			case alreadyKnown:
				fallthrough
			case replacementTransactionUnderpriced:
				results = append(results, getResult(account, StatusPending))
			default:
				results = append(results, getResult(account, StatusFail))
			}
			continue
		}

		timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelFunc()
		isMined, err := c.transactor.VerifyTx(timeoutCtx, erc20Tx.Hash().Hex())
		if err != nil {
			results = append(results, getResult(account, StatusFail))
			continue
		}
		if !isMined {
			results = append(results, getResult(account, StatusPending))
			continue
		}
		results = append(results, getResult(account, StatusSuccess))
	}

	return results
}

func getResult(account SourceAccount, status Status) Result {
	result := Result{
		SourceAccount: account,
		Status:        status,
	}
	return result
}

func (c evmCollector) hasTokenToCollect(ctx context.Context, toBeCollectedAccountAddr *common.Address, key SourceAccount) (bool, error) {
	accountToBeCollectedERC20Balance, err := c.transactor.BalanceOf(ctx, *toBeCollectedAccountAddr, key.Token)
	if err != nil {
		return false, err
	}

	if accountToBeCollectedERC20Balance.Cmp(big.NewInt(0)) == 0 {
		return false, err
	}
	return true, nil
}
