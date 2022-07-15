package dobermann

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"math/big"
	"time"
)

type Collector interface {
	Collect(ctx context.Context, collectionKey CollectionKey, keys []CollectionKey) ([]Result, error)
}

type Status string

const (
	StatusFail = "fail"
	StatusSuccess = "success"
	StatusPending = "pending"
	StatusSkip = "skip"
)

type Result struct {
	Status Status
	Hash   string
	Key    CollectionKey
}

type KmsKeyOptions struct {
	KeyID               string
	EncryptionAlgorithm types.EncryptionAlgorithmSpec
}

var _ Options = &KmsKeyOptions{}

type PrivateKeyOptions struct {
}

var _ Options = &PrivateKeyOptions{}

type Options interface {
}

type CollectionKey struct {
	Key        string
	KeyType    string
	KeyOptions Options
	Token      string
	Amount     string
}

func NewPolygonCollector(transactor Transactor, factory DecrypterFactory) Collector {
	return PolygonCollector{
		transactor: transactor,
		factory:    factory,
	}
}

type PolygonCollector struct {
	transactor Transactor
	factory    DecrypterFactory
}

func (k PolygonCollector) Collect(ctx context.Context, collectionKey CollectionKey, keys []CollectionKey) ([]Result, error) {
	decriptedCollectionKey, err := decryptKey(ctx, k.factory, collectionKey)
	if err != nil {
		return nil, err
	}

	var results []Result

	for _, key := range keys {
		decryptedKey, err := decryptKey(ctx, k.factory, key)
		if err != nil {
			return nil, err
		}

		result := Result{Key: key}

		toBeCollectedAccountAddr, err := k.transactor.GetAddressFromKey(decryptedKey)
		if err != nil {
			result.Status = StatusFail
			results = append(results, result)
			continue
		}

		hasTokenToCollect, err := k.hasTokenToCollect(ctx, toBeCollectedAccountAddr, key)
		if err != nil {
			result.Status = StatusFail
			results = append(results, result)
			continue
		}

		if !hasTokenToCollect {
			result.Status = StatusSkip
			results = append(results, result)
		}

		gasTipCapValue, gasFeeCapValue, err := k.transactor.GetGasCapValues(ctx)
		if err != nil {
			result.Status = StatusFail
			results = append(results, result)
			continue
		}
		log.Info().
			Str("gasTipCapValue", gasTipCapValue.String()).
			Str("gasFeeCapValue", gasFeeCapValue.String()).
			Msg("calculated fees")

		ecr20TxParams := TxParams{
			tokenAddr:      key.Token,
			senderKey:      decryptedKey,
			receiverKey:    decriptedCollectionKey,
			amount:         key.Amount,
			gasTipCapValue: gasTipCapValue,
			gasFeeCapValue: gasFeeCapValue,
		}
		erc20Tx, err := k.transactor.CreateERC20Tx(ctx, ecr20TxParams)
		if err != nil {
			result.Status = StatusFail
			results = append(results, result)
			continue
		}
		estimatedFee := new(big.Int).Add(new(big.Int).Mul(big.NewInt(int64(erc20Tx.Gas())), gasFeeCapValue), gasTipCapValue)
		accountToBeCollectedBalance, err := k.transactor.BalanceAt(ctx, *toBeCollectedAccountAddr)
		if err != nil {
			return nil, err
		}
		log.Info().
			Str("estimatedFee", estimatedFee.String()).
			Interface("accountToBeCollectedBalance", accountToBeCollectedBalance).
			Msg("calculated fees")
		if accountToBeCollectedBalance.Cmp(big.NewInt(0)) > 0 {
			remainingFee := new(big.Int).Sub(estimatedFee, accountToBeCollectedBalance)
			log.Info().
				Str("remainingFee", remainingFee.String()).
				Msg("calculated fees")
			if remainingFee.Cmp(big.NewInt(0)) >= 0 {
				nativTxParams := TxParams{
					senderKey:      decriptedCollectionKey,
					receiverKey:    decryptedKey,
					amount:         remainingFee.String(),
					gasTipCapValue: gasTipCapValue,
					gasFeeCapValue: gasFeeCapValue,
				}
				nativTx, err := k.transactor.CreateTx(ctx, nativTxParams)
				if err != nil {
					result.Status = StatusFail
					results = append(results, result)
					continue
				}

				err = k.transactor.Transfer(ctx, nativTx)
				if err != nil {
					result.Status = StatusFail
					results = append(results, result)
					continue
				}

				timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Minute)
				defer cancelFunc()
				isMined, err := k.transactor.VerifyTx(timeoutCtx, nativTx.Hash().Hex())
				if err != nil {
					result.Status = StatusFail
					results = append(results, result)
					continue
				}

				if !isMined {
					result.Status = StatusFail
					results = append(results, result)
					continue
				}
			}

		}
		log.Info().Str("tx", erc20Tx.Hash().Hex()).Msg("sending erc20 token tx")

		err = k.transactor.Transfer(ctx, erc20Tx)
		if err != nil {
			result.Status = StatusFail
			results = append(results, result)
			continue
		}

		timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelFunc()
		isMined, err := k.transactor.VerifyTx(timeoutCtx, erc20Tx.Hash().Hex())
		if err != nil {
			result.Status = StatusFail
			results = append(results, result)
			continue
		}
		if !isMined {
			result.Status = StatusPending
			results = append(results, result)
			continue
		}
		result.Status = StatusSuccess
		result.Hash = erc20Tx.Hash().Hex()
		results = append(results, result)
	}

	return results, err
}

func (k PolygonCollector) hasTokenToCollect(ctx context.Context, toBeCollectedAccountAddr *common.Address, key CollectionKey) (bool, error) {
	accountToBeCollectedERC20Balance, err := k.transactor.BalanceOf(ctx, *toBeCollectedAccountAddr, key.Token)
	if err != nil {
		return false, err
	}

	if accountToBeCollectedERC20Balance.Cmp(big.NewInt(0)) == 0 {
		return false, err
	}
	return true, nil
}

func decryptKey(ctx context.Context, factory DecrypterFactory, key CollectionKey) (string, error) {
	decrypter, err := factory.GetDecrypter(key.KeyType)(key.KeyOptions)
	if err != nil {
		return "", err
	}
	decryptedKey, err := decrypter.Decrypt(ctx, key.Key)
	if err != nil {
		return "", err
	}
	return decryptedKey, nil
}
