# Quickstart

Run a local Viri testnet in minutes using Docker Compose.

## Prerequisites

- Docker and Docker Compose
- Git

## Steps

```bash
# Clone the repository
git clone https://github.com/viri-chain/viri.git
cd viri

# Start the testnet
docker compose up -d

# Check validator logs
docker compose logs -f validator-0

# Monitor block production
docker compose logs --tail=20 bootstrap | grep "block"
```

## Verify

```bash
# Check block height
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Check peer count
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'

# View node status
curl http://localhost:8546/api/v1/status
```

## Network Details

| Parameter | Value |
|-----------|-------|
| Chain ID | 2 |
| RPC URL | http://localhost:8545 |
| REST API | http://localhost:8546 |
| Block Time | ~2.5 seconds |
| Validators | 4 |
| Currency | VIRI |
