# Viri Node Production Deployment Guide

## System Requirements

- **OS**: Linux (Ubuntu 22.04 LTS / Debian 12 / RHEL 9)
- **CPU**: 4+ cores
- **RAM**: 8 GB minimum, 16 GB recommended
- **Disk**: 100 GB SSD (NVMe preferred for validator nodes)
- **Network**: 100 Mbps+ with low latency (< 50ms to peers)

## Public Testnet Launch

### Bootstrap Peers

Before the testnet goes live, you need at least 2-3 bootstrap nodes with static IPs.

**Setting up a bootstrap node:**
```bash
# On each bootstrap machine:
virid --rpc --api \
  --config configs/node-testnet.json \
  --name "bootstrap-1"

# Note the peer ID printed at startup:
# PeerID: 16Uiu2HAm...
```

**Updating bootstrap peers in config:**
Edit `configs/node-testnet.json` and replace the placeholder values:
```json
"bootstrap_peers": [
  "/ip4/BOOTSTRAP_NODE_1_IP/tcp/30303/p2p/16Uiu2HAm...",
  "/ip4/BOOTSTRAP_NODE_2_IP/tcp/30303/p2p/16Uiu2HAm..."
]
```

### Genesis Ceremony

The genesis ceremony creates the initial validator set and chain configuration.

**On the coordinator node:**
```bash
# Initialize ceremony directory
virictl genesis init --dir .viri/genesis

# Add initial validators (run on each validator node)
virictl genesis contribute --dir .viri/genesis \
  --validator-key /path/to/validator.key

# Collect contributions and finalize
virictl genesis collect --dir .viri/genesis

# Export genesis file
virictl genesis export --dir .viri/genesis > genesis.json

# Distribute genesis.json to all testnet participants
```

**On each participant node:**
```bash
# Place genesis file
cp genesis.json configs/genesis/testnet.json

# Start node with testnet config
virid --config configs/node-testnet.json
```

### Testnet Quick Start

```bash
# 1. Build
git clone https://github.com/viri-chain/viri.git
cd viri
go build -o virid ./cmd/virid
go build -o virictl ./cmd/virictl

# 2. Set passphrase (required for first run)
export VIRI_KEY_PASSPHRASE=$(openssl rand -hex 32)

# 3. Generate genesis (or use distributed genesis file)
# See "Genesis Ceremony" above

# 4. Start node
./virid --config configs/node-testnet.json --rpc --api

# 5. Verify it's syncing
curl -s http://localhost:8545/health
```

### Auto-generated API Key

If `api_key_hash` is left empty in config and `--rpc` or `--api` is enabled, the node will:
1. Generate a random 32-byte API key
2. Save it to `{data_dir}/api_key.txt` (persisted across restarts)
3. Print the key on stderr at startup

```
=== API KEY ===
a1b2c3d4e5f6...
===============
```

This key never changes unless you delete `api_key.txt`. Set via header:
```bash
curl -H "X-API-Key: a1b2c3d4e5f6..." http://localhost:8545/ -X POST ...
```

## Quick Install

### 1. Build from Source

```bash
git clone https://github.com/viri-chain/viri.git
cd viri
go build -o virid ./cmd/virid
go build -o virictl ./cmd/virictl
sudo cp virid virictl /usr/local/bin/
```

### 2. Create System User

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin viri
sudo mkdir -p /var/lib/viri /etc/viri/tls /etc/viri/virid.env
sudo chown -R viri:viri /var/lib/viri /etc/viri
```

### 3. Generate TLS Certificates

```bash
# Self-signed (for testing) or use Let's Encrypt
sudo openssl req -x509 -nodes -days 365 \
  -newkey rsa:2048 \
  -keyout /etc/viri/tls/server.key \
  -out /etc/viri/tls/server.crt \
  -subj "/CN=viri-node"
sudo chmod 640 /etc/viri/tls/server.key
sudo chown viri:viri /etc/viri/tls/server.*
```

### 4. Generate API Key

```bash
# Generate a random API key
API_KEY=$(openssl rand -hex 32)
echo "API Key: $API_KEY"

# Hash it for the config
API_KEY_HASH=$(echo -n "$API_KEY" | sha256sum | awk '{print $1}')
# Note: the Go code hashes the raw bytes, so use this helper:
echo -n "$API_KEY" | sha256sum --binary | xxd -p -c 64
```

### 5. Configure Environment

```bash
sudo tee /etc/viri/virid.env > /dev/null << EOF
VIRI_KEY_PASSPHRASE=<strong-random-passphrase>
VIRI_TLS_CERT=/etc/viri/tls/server.crt
VIRI_TLS_KEY=/etc/viri/tls/server.key
VIRI_API_KEY_HASH=<sha256-hash-of-api-key>
VIRI_READINESS_MIN_PEERS=3
VIRI_READINESS_FORCE=0
VIRI_LOG_LEVEL=info
EOF

sudo chmod 600 /etc/viri/virid.env
sudo chown viri:viri /etc/viri/virid.env
```

### 6. Install Systemd Services

```bash
sudo cp virid.service /etc/systemd/system/
sudo cp virid-backup.service /etc/systemd/system/
sudo cp virid-backup.timer /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable virid.service
sudo systemctl enable virid-backup.timer
```

### 7. Configure Firewall

#### UFW (Ubuntu/Debian)

```bash
# Allow P2P connections (public)
sudo ufw allow 30303/tcp comment "Viri P2P"

# Allow SSH (if needed)
sudo ufw allow 22/tcp

# Block RPC/API from public access (only localhost)
sudo ufw deny 8545/tcp
sudo ufw deny 8546/tcp

# Enable firewall
sudo ufw enable
```

#### firewalld (RHEL/CentOS/Fedora)

```bash
sudo firewall-cmd --permanent --add-port=30303/tcp
sudo firewall-cmd --permanent --remove-port=8545/tcp
sudo firewall-cmd --permanent --remove-port=8546/tcp
sudo firewall-cmd --reload
```

#### iptables (direct)

```bash
# Allow P2P
iptables -A INPUT -p tcp --dport 30303 -j ACCEPT

# Allow RPC/API only from localhost
iptables -A INPUT -p tcp --dport 8545 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 8545 -j DROP
iptables -A INPUT -p tcp --dport 8546 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 8546 -j DROP

# Allow established connections
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
```

### 8. Start the Node

```bash
sudo systemctl start virid
sudo systemctl status virid
sudo journalctl -u virid -f
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VIRI_KEY_PASSPHRASE` | Keystore decryption passphrase | (required in prod) |
| `VIRI_TLS_CERT` | TLS certificate path | "" |
| `VIRI_TLS_KEY` | TLS private key path | "" |
| `VIRI_API_KEY_HASH` | SHA256 hash of API key | "" |
| `VIRI_READINESS_MIN_PEERS` | Minimum peers for ready state | 3 |
| `VIRI_READINESS_MIN_HEIGHT` | Minimum block height for ready | 0 |
| `VIRI_READINESS_FORCE` | Bypass readiness checks | 0 (dev: 1) |
| `VIRI_LOG_LEVEL` | Log level (debug/info/warn/error) | info |
| `VIRI_CHAIN_ID` | Chain ID | 1 |
| `VIRI_RPC_PORT` | RPC server port | 8545 |
| `VIRI_API_PORT` | API server port | 8546 |

### Config File

```json
{
  "chain": {
    "chain_id": 1,
    "network_name": "viri-mainnet",
    "block_time": "1s",
    "max_block_size": 10485760,
    "max_gas_per_block": 30000000
  },
  "network": {
    "max_peers": 50,
    "enable_dht": true
  },
  "node": {
    "validator_mode": true,
    "rpc_enabled": true,
    "rpc_port": 8545,
    "api_enabled": true,
    "api_port": 8546
  },
  "readiness": {
    "min_peers": 1,
    "min_block_height": 0,
    "force_ready": false
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

## API Usage

### With API Key

```bash
# Via header (recommended)
curl -H "X-API-Key: $API_KEY" https://localhost:8545/ \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Via Bearer token
curl -H "Authorization: Bearer $API_KEY" https://localhost:8545/ready

# Via query parameter (less secure)
curl "https://localhost:8545/ready?api_key=$API_KEY"
```

### CLI Tool

```bash
# Create backup
virictl backup create

# List backups
virictl backup list

# Restore from backup
sudo systemctl stop virid
virictl backup restore /var/lib/viri/backups/viri-backup-20250101-120000.tar.gz
sudo systemctl start virid

# Check status
virictl status --rpc https://localhost:8545
```

## Backup Management

### Manual Backup

```bash
virictl backup create --data-dir /var/lib/viri --backup-dir /var/lib/viri/backups
```

### Automated Backup

The systemd timer runs daily backups automatically:

```bash
# Check timer status
systemctl status virid-backup.timer

# Trigger immediate backup
systemctl start virid-backup.service

# View backup logs
journalctl -u virid-backup -f
```

### Backup Retention

Default: 10 backups (configurable via `--max-backups`).

## Monitoring

### Prometheus Metrics

Metrics are available at `http://127.0.0.1:8545/metrics` and `http://127.0.0.1:8546/metrics` (localhost only).

### Alerting Rules

Copy `monitoring/alerting_rules.yml` to your Prometheus configuration:

```yaml
rule_files:
  - "/etc/prometheus/rules/viri_alerting_rules.yml"
```

### Key Metrics

| Metric | Description |
|--------|-------------|
| `viri_chain_block_height` | Current blockchain height |
| `viri_p2p_peer_count` | Number of connected peers |
| `viri_service_ready` | Readiness state (1=ready) |
| `viri_http_requests_total` | HTTP request counts by status |
| `viri_http_request_duration_seconds` | Request latency histogram |
| `viri_http_in_flight_requests` | Currently processing requests |

## Security Checklist

- [ ] TLS certificates installed and configured
- [ ] API key hash set in environment
- [ ] `VIRI_KEY_PASSPHRASE` is a strong, unique passphrase
- [ ] Firewall blocks 8545/8546 from public access
- [ ] Systemd service hardening directives applied
- [ ] Daily backups configured and tested
- [ ] Monitoring and alerting configured
- [ ] Log rotation configured (audit logs auto-rotate)
- [ ] Node running as unprivileged `viri` user
- [ ] `NoNewPrivileges=yes` enforced by systemd

## Troubleshooting

### Node won't start

```bash
journalctl -u virid -n 100 --no-pager
```

### Check readiness

```bash
curl -s https://localhost:8545/ready | jq
```

### View audit logs

```bash
cat /var/lib/viri/logs/audit.log | jq
```

### Test connectivity

```bash
# P2P port (should be open)
nc -zv <node-ip> 30303

# RPC port (should be blocked from external)
nc -zv <node-ip> 8545
```

### Restore from backup

```bash
sudo systemctl stop virid
virictl backup restore /path/to/backup.tar.gz --data-dir /var/lib/viri
sudo systemctl start virid
```
