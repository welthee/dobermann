package key

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// Provider defines the methods needed to send and sign transactions
type Provider interface {
	// GetAddress returns an Address which contains the 20 byte address of an Ethereum account
	GetAddress() *common.Address
	// GetTransactOpts returns TransactOpts which contains the required data to be able
	// to sign an Ethereum transaction.
	GetTransactOpts() *bind.TransactOpts
}
