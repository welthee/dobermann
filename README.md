![dobermann_gopher](gopher.png)
# dobermann

### Description
Utility tool used to collect ERC-20 tokens in a specific account from other given accounts. 

The ERC-20 tokens have to be transferred from the collected account. 
In order to be able to execute a transaction on Ethereum (and other blockchains alike), you have to be able to pay for gas.
The tool makes the ERC-20 collection and also handles the gas.

First it transfers required gas from the destination account to the ERC-20 source account, and then transfers 
the ERC-20 tokens from the ERC-20 source account to the destination account.

To use the tool we have to provide the private keys for all accounts. They can be provided in plain of KMS encrypted.
### Configuration
#### transaction signing
This tool uses eth_sendRawTransaction method which sends to the network an already signed and serialized transaction.
In order to be able to sign transactions we need to manage our own keys. 

The destination account and each source account will need to have configured a key provider. 
We can provide the private key hex, a KMS encrypted private key hex, or we can provide a kms key and do the 
signing directly through KMS.

Key provider example:

```go
	keyProvider, _ := NewPrivateKeyProvider("privateKeyHex", "chainID")

	collectionKey := DestinationAccount{KeyProvider: keyProvider}
```

#### nonces

There are 2 nonce provider types which can be used: `NonceProviderTypeFixed` and `NonceProviderTypeNetwork`.

`NonceProviderTypeFixed` - returns a fixed configured value or default 0

`NonceProviderTypeNetwork` -  interrogates the network for the next nonce value

### Results

There are 4 possible outcomes: `StatusFail`, `StatusSuccess`, `StatusPending` , `StatusSkip` 

`StatusFail` - some error occurred and the collection could not be made.

`StatusSuccess` - collection made successfully 

`StatusPending` - collection was initiated but transaction could not be verified or collection already pending
and a replacement could not be made

`StatusSkip` - no funds available for transfer or another transfer was made successfully in the meantime 
