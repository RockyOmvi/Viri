# Public Testnet

The Viri public testnet is a 4-validator network running on Azure.

## Network Details

| Parameter | Value |
|-----------|-------|
| Network Name | Viri Testnet |
| Chain ID | 2 |
| RPC Endpoint | https://rpc.testnet.viri.me |
| REST API | https://api.testnet.viri.me |
| WebSocket | wss://ws.testnet.viri.me |
| Faucet | https://faucet.testnet.viri.me |
| Explorer | https://explorer.testnet.viri.me |
| Block Time | ~2.5 seconds |
| Validators | 4 |
| Consensus | HotStuff BFT |

## Faucet

Get testnet VIRI tokens from the [faucet](https://faucet.testnet.viri.me).

- Per claim: 10 VIRI
- Daily limit: 50 VIRI
- Cooldown: 24 hours

## Status

Check network status via:

```bash
# Health check
curl https://api.testnet.viri.me/api/v1/health

# Block height
curl https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Peer count
curl https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'
```
