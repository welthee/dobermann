package dobermann

import (
	"context"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	gasTrackerUrl = "https://gasstation-mumbai.matic.today/v2"
	blockchainUrl = "https://polygon-mumbai.infura.io/v3/18b346558fb545a586b9a7af4a1bab19"
)

func TestCollect(t *testing.T) {

	client, err := ethclient.Dial(blockchainUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	gasTracker := NewPolygonGasTracker(gasTrackerUrl)

	transactor, err := NewPolygonTransactor(client, &gasTracker)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	factory := NewDecrypterFactory(nil)

	worker := NewPolygonCollector(transactor, factory)

	key := CollectionKey{
		Key: "060ecff13c91ee46156397a6e8839f4f5c8e64e49f196bda7763ef30b616d5ce",
		//	Key:     "3fe974b1b2d50564632a9b0641be15216c0d1dc5a76e2e111dfd9d116c6c909a",
		KeyType:    "PK",
		KeyOptions: PrivateKeyOptions{},
		Token:      "0xc2ded4320937DAD8B46e5cDdB52823DD73458b84",
		Amount:     "10",
	}

	collectionKey := CollectionKey{
		Key:        "0d48dde10f9a973ff30e5c1363b57ad0b4f1faec8e2c9b3fc6de6aad07100650",
		KeyType:    "PK",
		KeyOptions: PrivateKeyOptions{},
	}

	result, err := worker.Collect(context.TODO(), collectionKey, []CollectionKey{key})
	if err != nil {
		log.Err(err).Msg("")
	}

	log.Info().Interface("result", result).Msg("got")
	assert.NotNil(t, result)
	assert.NotEqual(t, len(result), 0)
	for _, r := range result {
		assert.NotEmpty(t, r.Status)
	}
}
