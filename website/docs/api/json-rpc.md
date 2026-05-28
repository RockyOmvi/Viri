# JSON-RPC API

Viri supports standard Ethereum JSON-RPC methods plus custom Viri-specific methods.

## Standard Methods

### eth_blockNumber
Returns the latest block height.

```json
// Request
{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}
// Response
{"jsonrpc":"2.0","result":"0x1234","id":1}
```

### eth_getBlockByNumber
Returns block information by block number.

```json
// Request
{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x1",true],"id":1}
// Response
{"jsonrpc":"2.0","result":{"number":"0x1","hash":"0x...","parentHash":"0x...","timestamp":"0x...","miner":"0x...","transactions":[...],"gasUsed":"0x...","gasLimit":"0x...","stateRoot":"0x..."},"id":1}
```

### eth_getBlockByHash
Returns block information by block hash.

### eth_getTransactionByHash
Returns transaction details by transaction hash.

### eth_getTransactionReceipt
Returns transaction receipt by transaction hash.

### eth_getBalance
Returns the balance of an address.

```json
{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x...","latest"],"id":1}
```

### eth_sendRawTransaction
Submit a signed transaction to the network.

### eth_call
Execute a contract call locally (read-only).

### eth_estimateGas
Estimate gas for a transaction.

### eth_getLogs
Get event logs matching a filter.

### eth_gasPrice
Returns the current gas price (base fee).

### eth_chainId
Returns the chain ID.

### net_version
Returns the network version (chain ID as string).

### net_peerCount
Returns the number of connected peers.

## Viri-Specific Methods

### viri_nodeInfo
Returns detailed node information.

```json
// Request
{"jsonrpc":"2.0","method":"viri_nodeInfo","params":[],"id":1}
// Response
{"jsonrpc":"2.0","result":{"version":"0.1.0","chain_id":2,"peer_id":"...","full_peer_id":"...","multiaddr":"...","peers":4,"height":1000,"listening":true,"validator":true},"id":1}
```

### viri_getPeers
Returns the list of connected peers with their status.

### viri_getConsensusState
Returns the current consensus state (height, view, phase, reward_pool, epoch_start, last_finalized, validator_count, total_stake).

### viri_addPeer
Add a peer to the network by multiaddress.

## Account Abstraction Methods

### eth_sendUserOperation
Submit a user operation for ERC-4337 account abstraction.

### eth_estimateUserOperationGas
Estimate gas for a user operation.
