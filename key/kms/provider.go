package kms

import (
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/welthee/dobermann/key"
	ethawskmssigner "github.com/welthee/go-ethereum-aws-kms-tx-signer/v2"
	"math/big"
)

type kmsKeyProvider struct {
	TransactOpts *bind.TransactOpts
	Address      *common.Address
}

func (k kmsKeyProvider) GetAddress() *common.Address {
	return k.Address
}

func (k kmsKeyProvider) GetTransactOpts() *bind.TransactOpts {
	return k.TransactOpts
}

// NewKmsKeyProvider is a utility method to easily create a transaction signer
// using a KMS key for the given chainID.
func NewKmsKeyProvider(svc *kms.Client, keyId string, chainId *big.Int) (key.Provider, error) {
	txOpts, err := ethawskmssigner.NewAwsKmsTransactorWithChainID(svc, keyId, chainId)
	if err != nil {
		return nil, err
	}
	return kmsKeyProvider{
		TransactOpts: txOpts,
		Address:      &txOpts.From,
	}, nil
}
