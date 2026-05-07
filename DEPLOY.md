# Viri Blockchain - Testnet Deployment Guide

This guide covers deploying a Viri Blockchain testnet using Docker Compose, Kubernetes, or bare-metal servers.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start (Docker)](#quick-start-docker)
3. [Local Development Testnet](#local-development-testnet)
4. [Multi-Server Deployment](#multi-server-deployment)
5. [Kubernetes Deployment](#kubernetes-deployment)
6. [Monitoring Setup](#monitoring-setup)
7. [Validator Operations](#validator-operations)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### System Requirements (per validator)

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU      | 2 cores | 4+ cores    |
| RAM      | 4 GB    | 8+ GB       |
| Storage  | 50 GB   | 100+ GB SSD |
| Network  | 10 Mbps | 100+ Mbps   |
| Latency  | <100ms  | <50ms       |

### Software Requirements

- Go 1.21+ (for building from source)
- Docker 20.10+ and Docker Compose v2
- Kubernetes 1.24+ (for K8s deployment)
- Linux (Ubuntu 20.04+, Debian 11+, or RHEL 8+)

---

## Quick Start (Docker)

The fastest way to deploy a testnet is using the deployment script:

```bash
# Clone and build
git clone https://github.com/viri-chain/viri.git
cd viri

# Deploy 4-validator testnet with monitoring
make testnet-full

# Or manually
bash deploy/testnet-init.sh --validators 4 --monitoring --explorer
cd testnet && ./start.sh
```

**Access Points:**
- Validator RPC: http://localhost:8545
- Block Explorer: http://localhost:8080
- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9091

**Commands:**
```bash
cd testnet
./status.sh    # Check network status
./stop.sh      # Stop all nodes
./start.sh     # Start all nodes
```

---

## Local Development Testnet

For rapid local development with 4 validators:

```bash
# Build binaries first
make build

# Initialize testnet
bash deploy/testnet-init.sh \
  --validators 4 \
  --chain-id 1337 \
  --output-dir ./testnet

# Start the network
cd testnet && ./start.sh

# Interact with the network
curl http://localhost:8545/health
```

### Using Makefile

```bash
# Full deployment (build + init + start)
make testnet-full

# Custom configuration
make testnet-full VALIDATORS=6 CHAIN_ID=9999

# View logs
make testnet-logs

# Clean up
make testnet-clean
```

---

## Multi-Server Deployment

For production-like testnet across multiple servers:

### Step 1: Prepare Bootstrap Nodes

Deploy 2 bootstrap nodes first (can be smaller instances):

```bash
# On bootstrap server 1
docker run -d --name viri-bootstrap-1 \
  -p 30303:30303 \
  -v /data/viri:/home/viri/.viri \
  ghcr.io/viri-chain/viri:latest \
  virid --p2p-port 30303

# Get peer ID
docker logs viri-bootstrap-1 2>&1 | grep "peer_id"
```

### Step 2: Deploy Validator Nodes

On each validator server:

```bash
# 1. Generate validator key (on secure machine)
go run scripts/keygen.go > validator.key

# 2. Copy key to validator server securely
scp validator.key validator@node1:/etc/viri/

# 3. Provision the node
sudo bash deploy/scripts/provision-validator.sh \
  --key-file /etc/viri/validator.key \
  --peer "/ip4/BOOTSTRAP_IP/tcp/30303/p2p/BOOTSTRAP_PEER_ID" \
  --chain-id 1337 \
  --name "validator-0"
```

### Step 3: Genesis Ceremony

If using a custom genesis:

```bash
# Initialize ceremony
virictl genesis init --dir ~/.viri/genesis

# Add validators
virictl genesis add-validator \
  --name "validator-0" \
  --pubkey "PUBKEY_HEX" \
  --stake 1000000

# Each validator signs (offline recommended)
virictl genesis export-payload --dir ~/.viri/genesis
# Transfer payload to validator
virictl genesis sign --file payload.json

# Finalize
virictl genesis verify
virictl genesis finalize
virictl genesis export > genesis.json
```

---

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster with at least 4 nodes
- StorageClass provisioned (for PVCs)
- kubectl configured

### Deploy

```bash
# 1. Create namespace and apply manifests
kubectl apply -f deploy/k8s/deployment.yaml

# 2. Create validator keys secret
kubectl create secret generic viri-validator-keys \
  --from-file=validator-0.key=./keys/validator-0.key \
  --from-file=validator-1.key=./keys/validator-1.key \
  --from-file=validator-2.key=./keys/validator-2.key \
  --from-file=validator-3.key=./keys/validator-3.key \
  -n viri

# 3. Create genesis configmap
kubectl create configmap viri-genesis \
  --from-file=genesis.json=./genesis/genesis.json \
  -n viri

# 4. Verify deployment
kubectl get pods -n viri -w
kubectl get services -n viri
```

### Access Services

```bash
# Port-forward to RPC
kubectl port-forward -n viri svc/viri-rpc 8545:8545

# Port-forward to metrics
kubectl port-forward -n viri svc/viri-p2p 9090:9090
```

---

## Monitoring Setup

### Prometheus Configuration

The testnet deployment includes Prometheus scraping all validator metrics:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'viri-validators'
    static_configs:
      - targets:
          - 'validator-0:9090'
          - 'validator-1:9090'
          - 'validator-2:9090'
          - 'validator-3:9090'
```

### Available Metrics

| Metric | Description |
|--------|-------------|
| `viri_block_height` | Current block height |
| `viri_peer_count` | Number of connected peers |
| `viri_consensus_phase` | Current consensus phase |
| `viri_blocks_finalized_total` | Total finalized blocks |
| `viri_consensus_proposals_total` | Total proposals made |
| `viri_consensus_votes_total` | Total votes cast |
| `viri_mempool_size` | Transactions in mempool |
| `viri_tx_processed_total` | Total processed transactions |

### Grafana Dashboard

Access at http://localhost:3000 (default password: `admin`)

Pre-configured dashboards:
- **Viri Testnet Overview**: Block height, peer count, consensus status
- **Validator Performance**: CPU, memory, network I/O
- **Consensus Metrics**: Proposal rate, vote latency, view changes

---

## Validator Operations

### Adding a New Validator

1. Generate key pair:
```bash
virictl wallet create
```

2. Add to validator set (via governance proposal or genesis update)

3. Provision node:
```bash
sudo bash deploy/scripts/provision-validator.sh \
  --key-file ./validator.key \
  --peer "/ip4/BOOTSTRAP_IP/tcp/30303/p2p/BOOTSTRAP_PEER_ID"
```

### Removing a Validator

1. Unstake validator (via CLI):
```bash
virictl validator unstake --address 0x... --rpc http://localhost:8545
```

2. Stop the node:
```bash
sudo systemctl stop virid
sudo systemctl disable virid
```

### Backup and Restore

```bash
# Backup
virictl backup create --dir /var/lib/viri

# List backups
virictl backup list --dir /var/lib/viri

# Restore
virictl backup restore /var/lib/viri/backups/backup-2024-01-01.tar.gz
```

### Upgrading

```bash
# Docker
docker pull ghcr.io/viri-chain/viri:latest
cd testnet && ./stop.sh && ./start.sh

# Kubernetes
kubectl set image statefulset/viri-validator -n viri \
  validator=ghcr.io/viri-chain/viri:latest

# Systemd
sudo systemctl stop virid
sudo cp /tmp/new-virid /usr/local/bin/virid
sudo systemctl start virid
```

---

## Troubleshooting

### Node Won't Start

```bash
# Check logs
journalctl -u virid -e
docker logs viri-validator-0

# Verify genesis
cat testnet/genesis/genesis.json | jq

# Check config
cat testnet/configs/validator-0/config.json | jq
```

### No Peers Connected

```bash
# Check bootstrap peers in config
cat testnet/configs/validator-0/config.json | jq .network.bootstrap_peers

# Manually add peer
virictl peer add /ip4/PEER_IP/tcp/30303/p2p/PEER_ID --rpc http://localhost:8545

# List peers
virictl peer list --rpc http://localhost:8545
```

### Consensus Stuck

```bash
# Check validator status
virictl status --rpc http://localhost:8545

# Check if validator is in active set
curl http://localhost:8545/api/v1/validators

# Restart node
sudo systemctl restart virid
```

### Out of Disk Space

```bash
# Check chain data size
du -sh /var/lib/viri/chaindata

# Enable pruning (in config.json)
"storage": {
  "pruning_enabled": true,
  "pruning_keep_recent": 100000
}
```

### High Latency

```bash
# Check peer latencies
ping PEER_IP
traceroute PEER_IP

# Consider using dedicated bootstrap nodes in the same region
```

---

## Security Considerations

1. **Validator Keys**: Store in secure location, never commit to version control
2. **Firewall**: Only expose necessary ports (30303 for P2P, 8545 for RPC)
3. **TLS**: Enable TLS for RPC/API endpoints in production
4. **API Keys**: Use API key authentication for public RPC endpoints
5. **Monitoring**: Set up alerts for validator downtime and slashing events

---

## File Structure

```
deploy/
├── testnet-init.sh          # Main deployment script
├── entrypoint.sh            # Docker entrypoint
├── systemd/
│   └── virid.service        # Systemd unit file
├── k8s/
│   └── deployment.yaml      # Kubernetes manifests
└── scripts/
    └── provision-validator.sh  # Validator provisioning script

testnet/                     # Generated by testnet-init.sh
├── docker-compose.yml       # Generated compose file
├── genesis/
│   └── genesis.json         # Genesis configuration
├── keys/
│   └── validator-N.key      # Validator private keys
├── configs/
│   └── validator-N/
│       ├── config.json      # Node configuration
│       └── validator.key    # Validator key
├── start.sh                 # Start all nodes
├── stop.sh                  # Stop all nodes
├── status.sh                # Check network status
└── SUMMARY.txt              # Deployment summary
```
