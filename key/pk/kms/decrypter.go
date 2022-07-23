package kms

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// Decrypter defines methods used to encrypt and decrypt text with a KMS key
type Decrypter interface {
	// Decrypt decrypts ciphertext that was encrypted by a KMS key using RSAES_OAEP_SHA_256 algorithm
	Decrypt(ctx context.Context, data string) (string, error)
	// Encrypt encrypts plaintext with a KMS key using RSAES_OAEP_SHA_256 algorithm
	Encrypt(ctx context.Context, data string) (string, error)
}

type kmsDecrypter struct {
	svc   *kms.Client
	keyId string
}

func (k kmsDecrypter) Encrypt(ctx context.Context, data string) (string, error) {
	inputEncrypt := &kms.EncryptInput{
		KeyId:               aws.String(k.keyId),
		Plaintext:           []byte(data),
		EncryptionAlgorithm: types.EncryptionAlgorithmSpecRsaesOaepSha256,
	}
	respEncrypt, err := k.svc.Encrypt(ctx, inputEncrypt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(respEncrypt.CiphertextBlob), nil
}

func (k kmsDecrypter) Decrypt(ctx context.Context, data string) (string, error) {
	dataBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("unable to decode encryption data %s", data)
	}

	inputDecrypt := &kms.DecryptInput{
		CiphertextBlob:      dataBytes,
		KeyId:               aws.String(k.keyId),
		EncryptionAlgorithm: types.EncryptionAlgorithmSpecRsaesOaepSha256,
	}

	respDecrypt, err := k.svc.Decrypt(ctx, inputDecrypt)
	if err != nil {
		return "", err
	}

	return string(respDecrypt.Plaintext), nil
}

func NewKmsDecrypter(svc *kms.Client, keyId string) Decrypter {
	return kmsDecrypter{
		svc:   svc,
		keyId: keyId,
	}

}
