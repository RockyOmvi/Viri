# Viri Public Testnet

Welcome to the Viri public testnet! This guide will help you connect, claim test tokens, run a validator, and interact with the network.

## Network Details

| Parameter | Value |
|---|---|
| Chain ID | `1337` |
| Network Name | `viri-testnet` |
| Consensus | HotStuff BFT (PoS) |
| Block Time | 3 seconds |
| Validators | 4 (threshold: 3/4) |
| Native Token | VIRI (test) |
| Status | **Live** |

## Endpoints

### RPC (JSON-RPC)
```
https://rpc.viri-testnet.io:8545
```

### WebSocket
```
wss://rpc.viri-testnet.io:8546
```

### Explorer
```
https://explorer.viri-testnet.io:8080
```

### Faucet (get test tokens)
```
https://faucet.viri-testnet.io:8081
```

### Admin API (authenticated)
```
https://admin.viri-testnet.io:8546/admin/
```

## Quick Start

### 1. Install virictl

```bash
# Download latest binary
curl -LO https://github.com/viri-chain/viri/releases/latest/download/virictl-linux-amd64
chmod +x virictl-linux-amd64
sudo mv virictl-linux-amd64 /usr/local/bin/virictl

# Verify
virictl version
```

### 2. Connect to testnet

```bash
virictl connect --rpc https://rpc.viri-testnet.io:8545
```

### 3. Get test tokens

Visit the [faucet](https://faucet.viri-testnet.io:8081) and enter your wallet address:

```bash
# Or via CLI
virictl faucet claim --address <your-address> --faucet https://faucet.viri-testnet.io:8081
```

Limits: 100 VIRI per claim, 500 VIRI per day per address.

### 4. Check network status

```bash
virictl status --rpc https://rpc.viri-testnet.io:8545
virictl peers --rpc https://rpc.viri-testnet.io:8545
```

### 5. Send a transaction

```bash
virictl tx send \
  --from <your-address> \
  --to <recipient-address> \
  --amount 10 \
  --rpc https://rpc.viri-testnet.io:8545
```

## Running a Validator

### Requirements

- Linux VM (ARM64 or AMD64) with public IP
- Docker installed
- Port 30303 (P2P), 8545 (RPC), 8546 (API) open
- Minimum 10,000 VIRI stake

### Quick Start (Docker)

```bash
# 1. Set your passphrase (must be ≥12 chars)
export VIRI_KEY_PASSPHRASE=$(openssl rand -hex 32)

# 2. Generate validator key
docker run --rm \
  -v $(pwd)/config:/config \
  ghcr.io/viri-chain/viri:latest \
  virid --generate-validator-key --key-file /config/validator.key

# 3. Start validator
docker run -d \
  --name viri-validator \
  --restart unless-stopped \
  --network host \
  -v $(pwd)/data:/data \
  -v $(pwd)/config:/config \
  -e VIRI_CHAIN_ID=1337 \
  -e VIRI_NETWORK_NAME=viri-testnet \
  -e VIRI_KEY_PASSPHRASE=$VIRI_KEY_PASSPHRASE \
  -e VIRI_BOOTSTRAP_PEERS="/ip4/<bootstrap-ip>/tcp/30303/p2p/<peer-id>" \
  -e VIRI_VALIDATOR=true \
  -e VIRI_VALIDATOR_KEY=/config/validator.key \
  ghcr.io/viri-chain/viri:latest

# 4. Stake to register as validator
virictl validator stake \
  --amount 10000 \
  --rpc https://rpc.viri-testnet.io:8545

# 5. Check status
virictl validator status --rpc https://rpc.viri-testnet.io:8545
```

## Monitoring

### Prometheus Metrics

Each node exposes metrics at `:9090/metrics`:

- `viri_consensus_height` — Current block height
- `viri_consensus_view` — Current consensus view
- `viri_consensus_phase` — Current consensus phase
- `viri_consensus_validators` — Active validator count
- `viri_p2p_peers` — Connected peer count
- `viri_p2p_messages_total` — Total P2P messages
- `viri_mempool_size` — Pending transaction count

### Health Check

```bash
curl https://rpc.viri-testnet.io:8545/health
```

Response:
```json
{
  "status": "ok",
  "height": 12345,
  "peers": 3,
  "version": "1.0.0"
}
```

## Network Rules

- **Block time**: 3 seconds
- **Finality**: Instant (HotStuff BFT, 3/4 supermajority)
- **Minimum stake**: 10,000 VIRI
- **Slashing**: Downtime > 1 hour or double-signing
- **Epoch length**: 1,000 blocks (~50 minutes)
- **Max validators**: 100
- **Governance**: On-chain parameter voting (coming soon)

## Troubleshooting

### "No peers found"
Ensure your firewall allows inbound TCP on port 30303.

### "Insufficient stake"
Stake must be ≥ 10,000 VIRI. Use the faucet to get tokens first.

### "Node not syncing"
Check that your bootstrap peer address is correct and reachable.

### "Connection refused"
Verify the node is running: `docker ps | grep viri`

### "Passphrase error"
`VIRI_KEY_PASSPHRASE` must be at least 12 characters.

## Support

- GitHub Issues: https://github.com/viri-chain/viri/issues
- Discord: https://discord.gg/viri (coming soon)
- Explorer: https://explorer.viri-testnet.io:8080

## License

MIT
