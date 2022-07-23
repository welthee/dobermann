package dobermann

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/welthee/dobermann/key/pk"
	"testing"
)

const (
	gasTrackerUrl = "https://gasstation-mumbai.matic.today/v2"
	blockchainUrl = "https://polygon-mumbai.infura.io/v3/18b346558fb545a586b9a7af4a1bab19"
)

func TestCollect(t *testing.T) {

	config := EVMCollectorConfig{
		BlockchainUrl:     blockchainUrl,
		GasTrackerUrl:     gasTrackerUrl,
		NonceProviderType: NonceProviderTypeNetwork,
	}
	collector, err := NewEVMCollector(config)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	keyProvider, err := pk.NewPrivateKeyProvider("060ecff13c91ee46156397a6e8839f4f5c8e64e49f196bda7763ef30b616d5ce", collector.GetChainId(context.TODO()))
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	sourceAccount := SourceAccount{
		KeyProvider: keyProvider,
		Token:       "0xc2ded4320937DAD8B46e5cDdB52823DD73458b84",
		Amount:      "10",
	}
	collectionKeyProvider, err := pk.NewPrivateKeyProvider("0d48dde10f9a973ff30e5c1363b57ad0b4f1faec8e2c9b3fc6de6aad07100650",
		collector.GetChainId(context.TODO()))
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	collectionKey := DestinationAccount{
		KeyProvider: collectionKeyProvider,
	}

	result := collector.Collect(context.TODO(), collectionKey, []SourceAccount{sourceAccount})
	assert.NotNil(t, result)
	assert.NotEqual(t, len(result), 0)

	for _, r := range result {
		log.Info().Interface("result", r.Status).Msg("got")
		assert.NotEmpty(t, r.Status)
	}

}
