# WebSocket API

Viri supports WebSocket connections for real-time event streaming.

## Connection

```
ws://localhost:8545/ws
wss://rpc.testnet.viri.me/ws
```

## Events

### NewHeads
Subscribe to new block headers.

```json
{"jsonrpc":"2.0","method":"eth_subscribe","params":["newHeads"],"id":1}
```

### NewPendingTransactions
Subscribe to new pending transactions.

```json
{"jsonrpc":"2.0","method":"eth_subscribe","params":["newPendingTransactions"],"id":1}
```

### Logs
Subscribe to event logs matching a filter.

```json
{"jsonrpc":"2.0","method":"eth_subscribe","params":["logs",{"address":"0x..."}],"id":1}
```

## Unsubscribe

```json
{"jsonrpc":"2.0","method":"eth_unsubscribe","params":["0x1"],"id":1}
```
