# dobermann

Utility tool used to collect ERC-20 tokens in a specific wallet from other given wallets. 

The ERC-20 token has to be transferred from the collected account. 
In order be execute a transaction on Ethereum (and other blockchains alike), you have to be able to pay for gas.
The tool make the ERC-20 transfer and also handles the gas.

First it transfers required gas from the collection wallet to the collected wallet, and then transfers 
the ERC-20 tokens from the collected wallet to the collection wallet.

To use the tool we have to provide the private keys for all accounts. They can be provided in plain of KMS encrypted.


At [welthee](https://welthee.com) we are using AWS KMS managed private keys to sign Ethereum transactions.
