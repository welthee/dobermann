package main

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/welthee/dobermann"
	"github.com/welthee/dobermann/key/pk"
)

const (
	gasTrackerUrl = "https://gasstation-mumbai.matic.today/v2"
	blockchainUrl = "https://polygon-mumbai.infura.io/v3/18b346558fb545a586b9a7af4a1bab19"
)

func main() {

	config := dobermann.EVMCollectorConfig{
		BlockchainUrl:     blockchainUrl,
		GasTrackerUrl:     gasTrackerUrl,
		NonceProviderType: dobermann.NonceProviderTypeNetwork,
	}
	collector, err := dobermann.NewEVMCollector(config)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	fmt.Printf("Enter source accounts number: ")
	var accountsNo int

	_, err = fmt.Scanln(&accountsNo)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	sourceAccounts := make([]dobermann.SourceAccount, 0)

	for i := 0; i < accountsNo; i++ {
		fmt.Printf("Enter source private key: ")
		var key string

		_, err = fmt.Scanln(&key)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		keyProvider, err := pk.NewPrivateKeyProvider(key, collector.GetChainId(context.TODO()))
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		fmt.Printf("Enter token: ")
		var token string

		_, err = fmt.Scanln(&token)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		fmt.Printf("Enter amount in wei: ")
		var amount string

		_, err = fmt.Scanln(&amount)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		sourceAccount := dobermann.SourceAccount{
			KeyProvider: keyProvider,
			Token:       token,
			Amount:      amount,
		}

		sourceAccounts = append(sourceAccounts, sourceAccount)
	}

	fmt.Printf("Enter destination private key: ")
	var key string

	_, err = fmt.Scanln(&key)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	collectionKeyProvider, err := pk.NewPrivateKeyProvider(key,
		collector.GetChainId(context.TODO()))
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	collectionKey := dobermann.DestinationAccount{
		KeyProvider: collectionKeyProvider,
	}

	result := collector.Collect(context.TODO(), collectionKey, sourceAccounts)
	if len(result) == 0 {
		panic("panic")
	}

	for _, r := range result {
		log.Info().Interface("result", r.Status).Msg("got")
		if string(r.Status) == "" {
			panic("panic")
		}
	}

}
