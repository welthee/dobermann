package dobermann

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"math"
	"math/big"
	"strconv"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
)

type TxParams struct {
	tokenAddr string
	senderKey string
	receiverKey string
	amount string
	gasTipCapValue *big.Int
	gasFeeCapValue *big.Int
}

type Transactor interface {
	CreateERC20Tx(ctx context.Context, params TxParams) (*types.Transaction, error)
	CreateTx(ctx context.Context, params TxParams) (*types.Transaction, error)
	Transfer(ctx context.Context, transaction *types.Transaction) error
	VerifyTx(ctx context.Context, txHash string) (bool, error)
	BalanceAt(ctx context.Context, accountAddr common.Address) (*big.Int, error)
	BalanceOf(ctx context.Context, accountAddr common.Address, erc20Address string) (*big.Int, error)
	GetAddressFromKey(key string) (*common.Address, error)
	GetGasCapValues(ctx context.Context) (*big.Int, *big.Int, error)
}

type PolygonTransactor struct {
	client     *ethclient.Client
	gasTracker *PolygonGasTracker
}

func NewPolygonTransactor(client *ethclient.Client, tracker *PolygonGasTracker) (Transactor, error) {
	return PolygonTransactor{
		client:     client,
		gasTracker: tracker,
	}, nil

}
func (t PolygonTransactor) Transfer(ctx context.Context, transaction *types.Transaction) error {
	return t.client.SendTransaction(context.Background(), transaction)
}

func (t PolygonTransactor) CreateERC20Tx(ctx context.Context, params TxParams) (*types.Transaction, error) {
	senderAddress, err := t.GetAddressFromKey(params.senderKey)

	if err != nil {
		return nil, err
	}
	nonce, err := t.client.NonceAt(ctx, *senderAddress, nil)
	if err != nil {
		return nil, err
	}

	value := big.NewInt(0)
	receiverAddress, err := t.GetAddressFromKey(params.receiverKey)
	if err != nil {
		return nil, err
	}
	token := common.HexToAddress(params.tokenAddr)

	data := getTransactionData(*receiverAddress, params.amount)

	gasLimit, err := t.client.EstimateGas(ctx, ethereum.CallMsg{
		From: *senderAddress,
		To:   &token,
		Data: data,
	})
	if err != nil {
		return nil, err
	}

	feeTx := types.DynamicFeeTx{
		Nonce:     nonce,
		GasTipCap: params.gasTipCapValue,
		GasFeeCap: params.gasFeeCapValue,
		Gas:       gasLimit,
		To:        &token,
		Value:     value,
		Data:      data,
	}

	log.Info().
		Str("senderAddress", senderAddress.String()).
		Str("receiverAddress", receiverAddress.String()).
		Str("gasTipCapValue", params.gasTipCapValue.String()).
		Str("gasFeeCapValue", params.gasTipCapValue.String()).
		Str("amount", params.amount).
		Uint64("gasLimit", gasLimit).
		Uint64("nonce", nonce).
		Str("value", value.String()).
		Msg("details for erc20 tx")

	tx := types.NewTx(&feeTx)
	senderPrivateKey, err := crypto.HexToECDSA(params.senderKey)
	if err != nil {
		return nil, err
	}

	chainID, err := t.client.NetworkID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err = types.SignTx(tx, types.NewLondonSigner(chainID), senderPrivateKey)
	if err != nil {
		return nil, err
	}

	log.Info().Str("tx", tx.Hash().Hex()).Msg("created")
	return tx, nil
}

func (t PolygonTransactor) CreateTx(ctx context.Context, params TxParams) (*types.Transaction, error) {
	senderAddress, err := t.GetAddressFromKey(params.senderKey)
	if err != nil {
		return nil, err
	}

	nonce, err := t.client.NonceAt(ctx, *senderAddress, nil)
	if err != nil {
		return nil, err
	}
	value := new(big.Int)
	value.SetString(params.amount, 10)

	receiverAddress, err := t.GetAddressFromKey(params.receiverKey)
	if err != nil {
		return nil, err
	}

	var data []byte
	chainID, err := t.client.NetworkID(ctx)
	if err != nil {
		return nil, err
	}

	gasLimit, err := t.client.EstimateGas(ctx, ethereum.CallMsg{
		To:   receiverAddress,
		Data: data,
	})

	feeTx := types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: params.gasTipCapValue,
		GasFeeCap: params.gasFeeCapValue,
		Gas:       gasLimit,
		To:        receiverAddress,
		Value:     value,
		Data:      data,
	}
	log.Info().
		Str("senderAddress", senderAddress.String()).
		Str("receiverAddress", receiverAddress.String()).
		Str("gasTipCapValue", params.gasTipCapValue.String()).
		Str("gasFeeCapValue", params.gasTipCapValue.String()).
		Uint64("gasLimit", gasLimit).
		Uint64("nonce", nonce).
		Str("value", value.String()).
		Msg("details for tx")

	tx := types.NewTx(&feeTx)
	senderPrivateKey, err := crypto.HexToECDSA(params.senderKey)
	if err != nil {
		return nil, err
	}

	tx, err = types.SignTx(tx, types.NewLondonSigner(chainID), senderPrivateKey)
	if err != nil {
		return nil, err
	}
	log.Info().Str("tx", tx.Hash().Hex()).Msg("created")
	return tx, nil
}

func (t PolygonTransactor) VerifyTx(ctx context.Context, txHash string) (bool, error) {
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

func (t PolygonTransactor) BalanceAt(ctx context.Context, accountAddr common.Address) (*big.Int, error) {
	balance, err := t.client.BalanceAt(ctx, accountAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance wei: %w", err)
	}

	return balance, nil
}

func (t PolygonTransactor) BalanceOf(ctx context.Context, accountAddr common.Address, erc20Address string) (*big.Int, error) {
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

func (t PolygonTransactor) GetAddressFromKey(key string) (*common.Address, error) {
	privateKey, err := crypto.HexToECDSA(key)
	if err != nil {
		return nil, err
	}
	publicKey := privateKey.Public()
	publicKeyECDSA := publicKey.(*ecdsa.PublicKey)

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	return &fromAddress, nil
}

func (t PolygonTransactor) GetGasCapValues(ctx context.Context) (*big.Int, *big.Int, error) {
	gasTrackerResponse, err := t.gasTracker.getSuggestedGasPriceFromGasTracker(ctx)
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
