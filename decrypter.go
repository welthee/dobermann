package dobermann

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"strings"
)

type Decrypter interface {
	Decrypt(ctx context.Context, data string) (string, error)
	Encrypt(ctx context.Context, data string) (string, error)
}

type KmsDecrypter struct {
	svc           *kms.Client
	kmsKeyOptions KmsKeyOptions
}

func (k KmsDecrypter) Encrypt(ctx context.Context, data string) (string, error) {
	inputEncrypt := &kms.EncryptInput{
		KeyId:               aws.String(k.kmsKeyOptions.KeyID),
		Plaintext:           []byte(data),
		EncryptionAlgorithm: k.kmsKeyOptions.EncryptionAlgorithm,
	}
	respEncrypt, err := k.svc.Encrypt(ctx, inputEncrypt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(respEncrypt.CiphertextBlob), nil
}

func (k KmsDecrypter) Decrypt(ctx context.Context, data string) (string, error) {
	dataBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("unable to decode encryption data %s", data)
	}

	inputDecrypt := &kms.DecryptInput{
		CiphertextBlob:      dataBytes,
		KeyId:               aws.String(k.kmsKeyOptions.KeyID),
		EncryptionAlgorithm: k.kmsKeyOptions.EncryptionAlgorithm,
	}

	respDecrypt, err := k.svc.Decrypt(ctx, inputDecrypt)
	if err != nil {
		return "", err
	}

	return string(respDecrypt.Plaintext), nil
}

func NewKmsDecrypter(svc *kms.Client, options Options) Decrypter {
	return KmsDecrypter{
		svc:           svc,
		kmsKeyOptions: options.(KmsKeyOptions),
	}

}

type PKDecrypter struct {
	keyOptions PrivateKeyOptions
}

func (P PKDecrypter) Decrypt(ctx context.Context, data string) (string, error) {
	return data, nil
}

func (P PKDecrypter) Encrypt(ctx context.Context, data string) (string, error) {
	return data, nil
}

func NewPKDecrypter(options Options) Decrypter {
	return PKDecrypter{
		keyOptions: options.(PrivateKeyOptions),
	}

}

type DecrypterFactory struct {
	svc *kms.Client
}

func (u DecrypterFactory) GetDecrypter(kind string) func(options Options) (Decrypter, error) {
	switch strings.ToUpper(kind) {
	case "KMS":
		if u.svc == nil {
			return func(options Options) (Decrypter, error) {
				return nil, errors.New("unsupported decryption")
			}
		}
		return func(options Options) (Decrypter, error) {
			return NewKmsDecrypter(u.svc, options), nil
		}
	case "PK":
		return func(options Options) (Decrypter, error) {
			return NewPKDecrypter(options), nil
		}
	}

	return nil
}

func NewDecrypterFactory(svc *kms.Client) DecrypterFactory {
	return DecrypterFactory{
		svc: svc,
	}
}
