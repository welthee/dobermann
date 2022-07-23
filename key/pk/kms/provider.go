package kms

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/welthee/dobermann/key"
	"github.com/welthee/dobermann/key/pk"
	"math/big"
)

// NewKmsEncryptedPrivateKeyProvider is a utility method to easily create a transaction signer
// from a kms encrypted private key for the given chainID.
func NewKmsEncryptedPrivateKeyProvider(svc *kms.Client, kmsKeyId string, encryptedKey string, chainId *big.Int) (key.Provider, error) {
	decrypter := NewKmsDecrypter(svc, kmsKeyId)
	privateKeyHex, err := decrypter.Decrypt(context.TODO(), encryptedKey)
	if err != nil {
		return nil, err
	}
	return pk.NewPrivateKeyProvider(privateKeyHex, chainId)
}
