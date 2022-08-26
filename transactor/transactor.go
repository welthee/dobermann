package transactor

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/welthee/dobermann/key"
	"github.com/welthee/dobermann/nonce"
	"math"
	"math/big"
	"strconv"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
)

type TxParams struct {
	// ERC-20 token address
	TokenAddr string
	// sender account and gas provider
	SenderKeyProvider key.Provider
	// receiver of the ERC-20 token
	ReceiverKeyProvider key.Provider
	// amount sent in wei
	Amount string
	// maxPriorityFeePerGas
	GasTipCapValue *big.Int
	// maxFeePerGas
	GasFeeCapValue *big.Int
}

// Transactor contains methods needed to send and verify transactions
type Transactor interface {
	//CreateERC20Tx creates a signed ERC-20 tx using the provided TxParams params
	CreateERC20Tx(ctx context.Context, params TxParams) (*types.Transaction, error)
	//CreateTx creates a signed native tx using the provided TxParams params
	CreateTx(ctx context.Context, params TxParams) (*types.Transaction, error)
	//Transfer sends transaction to network
	Transfer(ctx context.Context, transaction *types.Transaction) error
	//VerifyTx checks if transaction is mined using the given transaction hash
	VerifyTx(ctx context.Context, txHash string) (bool, error)
	//BalanceAt returns the wei balance of the given account taken from the latest known block
	BalanceAt(ctx context.Context, accountAddr common.Address) (*big.Int, error)
	//BalanceOf returns the ERC-20 wei balance of the given account
	BalanceOf(ctx context.Context, accountAddr common.Address, erc20Address string) (*big.Int, error)
	//GetGasCapValues retrieves the network's suggested gas price
	GetGasCapValues(ctx context.Context) (*big.Int, *big.Int, error)
}

type evmTransactor struct {
	client        *ethclient.Client
	gasTracker    GasTracker
	nonceProvider nonce.Provider
}

// NewEvmTransactor utility method to create a EVM transactor
func NewEvmTransactor(client *ethclient.Client, tracker GasTracker, nonceProvider nonce.Provider) (Transactor, error) {
	return evmTransactor{
		client:        client,
		gasTracker:    tracker,
		nonceProvider: nonceProvider,
	}, nil

}
func (t evmTransactor) Transfer(ctx context.Context, transaction *types.Transaction) error {
	return t.client.SendTransaction(context.Background(), transaction)
}

func (t evmTransactor) CreateERC20Tx(ctx context.Context, params TxParams) (*types.Transaction, error) {
	senderAddress := *params.SenderKeyProvider.GetAddress()

	nonce, err := t.nonceProvider.GetNonce(ctx, params.SenderKeyProvider.GetAddress())
	if err != nil {
		return nil, err
	}
	value := big.NewInt(0)
	receiverAddress := *params.ReceiverKeyProvider.GetAddress()
	token := common.HexToAddress(params.TokenAddr)
	data := getTransactionData(receiverAddress, params.Amount)

	gasLimit, err := t.client.EstimateGas(ctx, ethereum.CallMsg{
		From: senderAddress,
		To:   &token,
		Data: data,
	})
	if err != nil {
		return nil, err
	}

	feeTx := types.DynamicFeeTx{
		Nonce:     nonce.Uint64(),
		GasTipCap: params.GasTipCapValue,
		GasFeeCap: params.GasFeeCapValue,
		Gas:       gasLimit,
		To:        &token,
		Value:     value,
		Data:      data,
	}

	tx := types.NewTx(&feeTx)
	transactOpts := params.SenderKeyProvider.GetTransactOpts()
	tx, err = transactOpts.Signer(transactOpts.From, tx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (t evmTransactor) CreateTx(ctx context.Context, params TxParams) (*types.Transaction, error) {
	senderAddress := params.SenderKeyProvider.GetAddress()
	receiverAddress := params.ReceiverKeyProvider.GetAddress()

	nonce, err := t.nonceProvider.GetNonce(ctx, senderAddress)
	if err != nil {
		return nil, err
	}

	value := new(big.Int)
	value.SetString(params.Amount, 10)

	var data []byte

	gasLimit, err := t.client.EstimateGas(ctx, ethereum.CallMsg{
		To:   receiverAddress,
		Data: data,
	})
	if err != nil {
		return nil, err
	}

	feeTx := types.DynamicFeeTx{
		Nonce:     nonce.Uint64(),
		GasTipCap: params.GasTipCapValue,
		GasFeeCap: params.GasFeeCapValue,
		Gas:       gasLimit,
		To:        receiverAddress,
		Value:     value,
		Data:      data,
	}

	tx := types.NewTx(&feeTx)

	transactOpts := params.SenderKeyProvider.GetTransactOpts()
	tx, err = transactOpts.Signer(transactOpts.From, tx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (t evmTransactor) VerifyTx(ctx context.Context, txHash string) (bool, error) {
	_, ok := ctx.Deadline()
	if !ok {
		return false, errors.New("context deadline not set")
	}

	if txHash == "" {
		return false, errors.New("tx is empty")
	}

	queryTicker := time.NewTicker(10 * time.Second)
	defer queryTicker.Stop()

	for {
		receipt, err := t.client.TransactionReceipt(ctx, common.HexToHash(txHash))
		if receipt != nil {
			if receipt.Status != 1 {
				return false, nil
			}
			log.Ctx(ctx).Debug().Msgf("found transaction receipt for tx=%s: status=%d", txHash, receipt.Status)
			return true, nil
		}
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Str("tx", txHash).Msg("failed to get receipt for tx")
		}

		select {
		case <-ctx.Done():
			log.Ctx(ctx).Warn().Err(ctx.Err()).Str("tx", txHash).Msg("failed to get receipt status")
			return false, ctx.Err()
		case <-queryTicker.C:
		}
	}

}

func (t evmTransactor) BalanceAt(ctx context.Context, accountAddr common.Address) (*big.Int, error) {
	balance, err := t.client.BalanceAt(ctx, accountAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance wei: %w", err)
	}

	return balance, nil
}

func (t evmTransactor) BalanceOf(ctx context.Context, accountAddr common.Address, erc20Address string) (*big.Int, error) {
	caller, err := NewIERC20Caller(common.HexToAddress(erc20Address), t.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get IERC20Caller: %w", err)
	}

	balance, err := caller.BalanceOf(nil, accountAddr)
	if err != nil {
		return nil, err
	}

	return balance, nil
}

func (t evmTransactor) GetGasCapValues(ctx context.Context) (*big.Int, *big.Int, error) {
	gasTrackerResponse, err := t.gasTracker.GetSuggestedGasPrice(ctx)
	if err != nil {
		return nil, nil, err
	}

	gasTipCapValue, ok := new(big.Int).SetString(formatFloat(gasTrackerResponse.SafeLow.MaxPriorityFee, 9), 10)
	if !ok {
		return nil, nil, errors.New("invalid gasTipCapValue")
	}
	gasFeeCapValue, ok := new(big.Int).SetString(formatFloat(gasTrackerResponse.SafeLow.MaxFee, 9), 10)
	if !ok {
		return nil, nil, errors.New("invalid gasFeeCapValue")
	}
	return gasTipCapValue, gasFeeCapValue, nil
}

func getTransactionData(toAddress common.Address, amountWei string) []byte {
	transferFnSignature := []byte("transfer(address,uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(transferFnSignature)
	methodID := hash.Sum(nil)[:4]

	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)

	amount := new(big.Int)
	amount.SetString(amountWei, 10)

	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)
	return data
}

func formatFloat(num float64, decimal int) string {
	d := float64(1)
	if decimal > 0 {
		d = math.Pow10(decimal)
	}
	return strconv.FormatFloat(math.Round(num*d), 'f', -1, 64)
}
