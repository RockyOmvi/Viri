# Configuration

Node configuration is stored in JSON files.

## Node Config

```json
{
  "node": {
    "id": "validator-0",
    "role": "validator",
    "data_dir": "/home/viri/.viri/data",
    "listen_addr": "/ip4/0.0.0.0/tcp/4000",
    "rpc_addr": ":8545",
    "api_addr": ":8546",
    "bootstrap_addrs": ["/ip4/.../tcp/4000/p2p/..."],
    "peers": 3,
    "min_peers": 3
  },
  "consensus": {
    "block_time": "2.5s",
    "view_timeout": "3s",
    "min_validators": 4
  },
  "storage": {
    "backend": "leveldb",
    "data_dir": "/home/viri/.viri/data"
  },
  "logging": {
    "level": "info",
    "output": "",
    "format": "text"
  }
}
```

## Genesis Config

```json
{
  "chain_id": 2,
  "network": "viri-testnet",
  "genesis_time": "2024-01-01T00:00:00Z",
  "block_time": "2.5s",
  "initial_supply": 10000000,
  "max_block_size": 1048576,
  "max_gas_per_block": 10000000,
  "initial_validators": [
    {"address": "0x...", "public_key": "0x...", "stake": 1000000, "name": "validator-0"}
  ]
}
```

## Key Configuration Files

| File | Description |
|------|-------------|
| `config/testnet/config.json` | Testnet genesis config |
| `config/mainnet/config.json` | Mainnet genesis config (future) |
| `configs/node-testnet.json` | Default node config |
| `testnet/configs/validator-N/config.json` | Per-validator configs |
| `testnet/genesis/genesis.json` | Testnet genesis state |
| `testnet/keys/validator-N.key` | secp256k1 private keys (32 bytes hex) |
