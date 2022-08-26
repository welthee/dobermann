package nonce

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

// Provider defines method to get a nonce value
type Provider interface {
	// GetNonce returns the nonce which will be associated with an account.
	GetNonce(ctx context.Context, address *common.Address) (*big.Int, error)
}

type fixedNonceProvider struct {
	nonce *big.Int
}

func (f fixedNonceProvider) GetNonce(ctx context.Context, address *common.Address) (*big.Int, error) {
	return f.nonce, nil
}

// NewFixedNonceProvider utility method to create a nonce provider which will
// return a fixed nonce value
func NewFixedNonceProvider(nonce *big.Int) Provider {
	if nonce != nil {
		return fixedNonceProvider{nonce: nonce}
	}

	return fixedNonceProvider{nonce: big.NewInt(0)}
}

type networkNonceProvider struct {
	client *ethclient.Client
}

func (f networkNonceProvider) GetNonce(ctx context.Context, address *common.Address) (*big.Int, error) {
	nonce, err := f.client.NonceAt(ctx, *address, nil)
	if err != nil {
		return nil, err
	}
	return big.NewInt(int64(nonce)), err
}

// NewNetworkNonceProvider utility method to create a nonce provider which will
// interrogate the network for the nonce value
func NewNetworkNonceProvider(client *ethclient.Client) Provider {
	return networkNonceProvider{client: client}
}
