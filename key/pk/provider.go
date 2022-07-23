package pk

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/welthee/dobermann/key"
	"math/big"
)

// NewPrivateKeyProvider is a utility method to easily create a transaction signer
// from a single private key for the given chainID.
func NewPrivateKeyProvider(privateKeyHex string, chainID *big.Int) (key.Provider, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}

	opts, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, err
	}
	return privateKeyProvider{
		TransactOpts: opts,
		Address:      &opts.From,
	}, nil
}

type privateKeyProvider struct {
	TransactOpts *bind.TransactOpts
	Address      *common.Address
}

func (p privateKeyProvider) GetAddress() *common.Address {
	return p.Address
}

func (p privateKeyProvider) GetTransactOpts() *bind.TransactOpts {
	return p.TransactOpts
}
