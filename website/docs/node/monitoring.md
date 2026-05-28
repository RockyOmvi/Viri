# Monitoring

## Prometheus Metrics

Viri exposes Prometheus metrics at `/metrics` on port 9090 (localhost only by default).

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `consensus_block_finalized_total` | Counter | Total blocks finalized |
| `consensus_height` | Gauge | Current block height |
| `consensus_view` | Gauge | Current consensus view |
| `consensus_phase` | Gauge | Current phase (0=idle, 1=prepare, 2=precommit, 3=commit, 4=decide) |
| `consensus_validators` | Gauge | Number of active validators |
| `p2p_peers_connected` | Gauge | Connected peers |
| `mempool_pending_txs` | Gauge | Pending transactions in mempool |
| `node_is_syncing` | Gauge | Whether node is syncing (1=true, 0=false) |

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
