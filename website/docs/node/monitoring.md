# Monitoring

## Prometheus Metrics

Viri exposes Prometheus metrics at `/metrics` on port 9090 (localhost only by default).

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `viri_blocks_total` | Counter | Total blocks produced |
| `viri_block_height` | Gauge | Current block height |
| `viri_peers` | Gauge | Connected peers |
| `viri_txs_total` | Counter | Total transactions |
| `viri_gas_price` | Gauge | Current base fee |
| `viri_consensus_view` | Gauge | Current consensus view |
| `viri_consensus_round` | Gauge | Current consensus round |

## Health Endpoints

```bash
# Node health
curl http://localhost:8545/health

# REST API health
curl http://localhost:8546/api/v1/health

# Ready check
curl http://localhost:8545/ready
```

## Logs

```bash
# Follow node logs
docker compose logs -f validator-0

# Search for errors
docker compose logs validator-0 | grep "error"

# Check recent blocks
docker compose logs validator-0 | grep "finalized"
```

## Grafana Dashboard

Import the dashboard from `monitoring/grafana/dashboard.json` for visual monitoring of:

- Block production rate
- Peer connectivity
- Consensus state
- Transaction volume
- Gas prices
- Node resource usage
