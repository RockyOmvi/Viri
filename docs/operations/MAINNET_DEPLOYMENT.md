# Mainnet Deployment Guide

This document covers the infrastructure setup, security hardening, and operational procedures for Viri mainnet validator nodes.

## Network Topology

```
                    Internet
                        |
                  +-----+------+
                  |  Load      |
                  |  Balancer  |
                  | (RPC/API)  |
                  +-----+------+
                        |
         +--------------+--------------+
         |              |              |
   +-----+------+  +---+------+  +----+-----+
   |  Sentry    |  |  Sentry   |  |  Sentry  |
   |  Node 1    |  |  Node 2   |  |  Node 3  |
   +-----+------+  +---+------+  +----+-----+
         |              |              |
         +--------------+--------------+
                        |
                +-------+--------+
                |   Validator    |
                |   Node (HIDDEN)|
                +----------------+
```

### Architecture Rules

- **Validator node never connects directly to the internet** — only to sentry nodes
- **Sentry nodes are DDoS buffers** — 3+ sentry nodes per validator, geographically distributed
- **RPC/API endpoints go through load balancers** — never expose validator ports directly
- **P2P traffic (30303) is the only public port** — RPC (8545) and API (8546) are localhost-only

---

## 1. Infrastructure Requirements

### Validator Node (Minimum)

| Component | Requirement | Recommended |
|-----------|-------------|-------------|
| CPU | 4 cores | 8+ cores (AMD EPYC / Intel Xeon) |
| RAM | 8 GB | 16 GB ECC |
| Disk | 100 GB SSD | 500 GB NVMe SSD |
| Network | 100 Mbps | 1 Gbps, <10ms to sentry nodes |
| OS | Ubuntu 22.04 LTS | Ubuntu 22.04 LTS / Debian 12 |
| Power | N/A | Dual PSU, UPS backup |

### Sentry Node

| Component | Requirement | Recommended |
|-----------|-------------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 50 GB SSD | 100 GB SSD |
| Network | 100 Mbps | 1 Gbps |
| Bandwidth | 2 TB/month | 5 TB/month (unmetered) |

### Seed/Bootstrap Node

| Component | Requirement | Recommended |
|-----------|-------------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 200 GB SSD | 500 GB SSD |
| Network | 100 Mbps | 1 Gbps |
| Static IP | Yes | Yes, with reverse DNS |

### Monitoring Node

| Component | Requirement |
|-----------|-------------|
| CPU | 2 cores |
| RAM | 4 GB |
| Disk | 100 GB SSD |
| Software | Prometheus, Grafana, Alertmanager |

---

## 2. Hardware Security Module (HSM) Setup

Validator signing keys should be stored in an HSM or secure enclave.

### Option A: YubiHSM2

```bash
# 1. Install YubiHSM SDK
sudo apt install yubihsm-connector yubihsm-shell

# 2. Connect YubiHSM and initialize
yubihsm-shell
> connect
> session open 1 <password>
> put object 0 0 generate_asymmetric_key 0x1000 "validator-key" 1 1 "ec-k1:secp256k1"

# 3. Extract public key
> get public_key 0 0x1000

# 4. Configure virid to use HSM
# Set in node config:
# "hsm": {
#   "enabled": true,
#   "module": "yubihsm",
#   "key_id": "0x1000",
#   "connector_url": "http://localhost:12345"
# }
```

### Option B: AWS CloudHSM / Azure Dedicated HSM

```bash
# 1. Create PKCS#11 provider config
cat > /etc/viri/hsm-pkcs11.conf << EOF
{
  "library": "/opt/cloudhsm/lib/libcloudhsm_pkcs11.so",
  "slot": 0,
  "pin": "<crypto-user-pin>",
  "key_label": "validator-signing-key"
}
EOF

# 2. Configure virid
# Set in node config:
# "hsm": {
#   "enabled": true,
#   "pkcs11_config": "/etc/viri/hsm-pkcs11.conf"
# }
```

### Option C: Software Key (air-gapped)

For validators without HSM access, the encrypted keystore file must be protected:

```bash
# Encrypt validator key with strong passphrase
# (handled automatically by virictl wallet create)

# Store passphrase in VIRI_KEY_PASSPHRASE env var
# This is loaded at process start and never written to disk
sudo tee /etc/viri/virid.env > /dev/null << EOF
VIRI_KEY_PASSPHRASE=<64-char-random-passphrase>
VIRI_TLS_CERT=/etc/viri/tls/server.crt
VIRI_TLS_KEY=/etc/viri/tls/server.key
EOF

sudo chmod 600 /etc/viri/virid.env
```

---

## 3. Validator Setup Guide

### Step 1: Provision the Machine

```bash
# Update system
sudo apt update && sudo apt upgrade -y
sudo apt install -y ufw jq htop iotop netdata

# Create viri user
sudo useradd --system --no-create-home --shell /usr/sbin/nologin viri
sudo mkdir -p /var/lib/viri /etc/viri/tls /etc/viri/keys
sudo chown -R viri:viri /var/lib/viri /etc/viri
```

### Step 2: Install Viri Binaries

```bash
# Build from source
git clone https://github.com/viri-chain/viri.git
cd viri
go build -o virid ./cmd/virid
go build -o virictl ./cmd/virictl
sudo cp virid virictl /usr/local/bin/

# Or download pre-built binary
# wget https://github.com/viri-chain/viri/releases/download/v0.1.0/virid-linux-amd64.tar.gz
# tar -xzf virid-linux-amd64.tar.gz
# sudo cp virid virictl /usr/local/bin/
```

### Step 3: Generate Validator Key

```bash
# Generate key on an air-gapped machine
virictl wallet create

# Transfer encrypted key file to the validator node
# (via encrypted USB or secure transfer)
sudo cp validator.key.enc /etc/viri/keys/
sudo chown viri:viri /etc/viri/keys/validator.key.enc

# Or generate directly (less secure):
export VIRI_KEY_PASSPHRASE=$(openssl rand -hex 32)
virictl wallet create
```

### Step 4: Set Up Sentry Node

```bash
# On each sentry machine:

# 1. Install and configure virid
# 2. Open P2P port
sudo ufw allow 30303/tcp comment "Viri P2P"

# 3. Block RPC/API from external
sudo ufw deny 8545/tcp
sudo ufw deny 8546/tcp

# 4. Allow access from validator IP only
sudo ufw allow from <validator-private-ip> to any port 8545 proto tcp
sudo ufw allow from <validator-private-ip> to any port 8546 proto tcp

# 5. Configure sentry node (node-sentry.json):
# {
#   "node": {
#     "validator_mode": false,
#     "rpc_enabled": false,
#     "api_enabled": false
#   },
#   "network": {
#     "max_peers": 200,
#     "enable_dht": true
#   }
# }

# 6. Start sentry
sudo systemctl start virid
```

### Step 5: Connect Validator to Sentry Nodes

```bash
# On validator node:

# 1. Configure to connect only to sentry nodes
# Edit node-mainnet.json:
# "network": {
#   "bootstrap_peers": [
#     "/ip4/<sentry-1-private-ip>/tcp/30303/p2p/<sentry-1-peer-id>",
#     "/ip4/<sentry-2-private-ip>/tcp/30303/p2p/<sentry-2-peer-id>",
#     "/ip4/<sentry-3-private-ip>/tcp/30303/p2p/<sentry-3-peer-id>"
#   ],
#   "max_peers": 50,
#   "enable_dht": false  # Disable DHT - only connect to sentries
# }

# 2. Enable validator mode
# "node": {
#   "validator_mode": true,
#   "validator_key": "/etc/viri/keys/validator.key.enc"
# }

# 3. Close ALL external ports on validator
sudo ufw default deny incoming
sudo ufw allow from <sentry-1-private-ip> to any port 30303 proto tcp
sudo ufw allow from <sentry-2-private-ip> to any port 30303 proto tcp
sudo ufw allow from <sentry-3-private-ip> to any port 30303 proto tcp
sudo ufw allow 22/tcp  # SSH access
sudo ufw enable
```

### Step 6: Generate TLS Certificates

```bash
# Production: Use Let's Encrypt
sudo apt install certbot
sudo certbot certonly --standalone -d validator-01.example.com
sudo ln -s /etc/letsencrypt/live/validator-01.example.com/fullchain.pem /etc/viri/tls/server.crt
sudo ln -s /etc/letsencrypt/live/validator-01.example.com/privkey.pem /etc/viri/tls/server.key
sudo chmod 640 /etc/viri/tls/server.key
sudo chown viri:viri /etc/viri/tls/server.*

# Development: Self-signed
sudo openssl req -x509 -nodes -days 365 \
  -newkey rsa:2048 \
  -keyout /etc/viri/tls/server.key \
  -out /etc/viri/tls/server.crt \
  -subj "/CN=viri-node"
```

### Step 7: Configure Environment

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

### Step 8: Install Systemd Service

```bash
# Copy service files from repository
sudo cp deploy/systemd/virid.service /etc/systemd/system/
sudo cp deploy/systemd/virid-backup.service /etc/systemd/system/
sudo cp deploy/systemd/virid-backup.timer /etc/systemd/system/

# Reload and enable
sudo systemctl daemon-reload
sudo systemctl enable virid.service
sudo systemctl enable virid-backup.timer

# Start the node
sudo systemctl start virid

# Verify
sudo systemctl status virid
journalctl -u virid -f
```

---

## 4. How to Join as a New Validator After Genesis

```bash
# 1. Set up infrastructure (sentry nodes, validator node)
#    following the steps above

# 2. Generate validator key
virictl wallet create

# 3. Acquire the minimum stake
#    (config: consensus.min_stake = 10,000,000 VIRI)

# 4. Submit a validator registration transaction
#    via governance or direct staking contract:
virictl validator register \
  --address <validator-address> \
  --pubkey <validator-public-key> \
  --amount 10000000

# 5. Wait for the next validator set rotation
#    (config: consensus.epoch_length = 1000 blocks)

# 6. Once included in the validator set, start your node:
sudo systemctl start virid

# 7. Verify you are producing blocks:
curl -s -H "X-API-Key: $API_KEY" http://localhost:8545/ \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_getConsensusState","params":[],"id":1}' \
  | jq
```

---

## 5. How to Safely Update a Validator Node

### Standard Update (Non-Consensus)

```bash
# 1. Build new binary
git pull
go build -o virid-new ./cmd/virid
go build -o virictl-new ./cmd/virictl

# 2. Verify binary hash
sha256sum virid-new

# 3. Check changelog for breaking changes
#    If no consensus-breaking changes:
sudo systemctl stop virid
sudo cp virid-new /usr/local/bin/virid
sudo cp virictl-new /usr/local/bin/virictl
sudo systemctl start virid

# 4. Verify upgrade
journalctl -u virid -n 50 | grep "upgrade\|version\|started"
```

### Consensus-Breaking Upgrade (Coordinated)

```bash
# 1. Signal intention to upgrade
#    All validators agree on upgrade block height

# 2. All validators set the upgrade block:
#    "consensus": {
#      "upgrade_block": 1000000,
#      "upgrade_version": "0.2.0"
#    }

# 3. Build and install new binary (same as above, do NOT restart)

# 4. The upgrade activates automatically at the agreed block
#    - Old binary stops producing blocks at that height
#    - New binary takes over
#    - NO restart needed — the binary detects the upgrade block
#      during normal operation

# 5. Monitor:
watch -n 5 'curl -s http://localhost:8545/health | jq .height'
```

---

## 6. DDoS Protection

### Sentry Architecture

The primary DDoS defense is the sentry architecture:
- Validator only talks to sentry nodes over private IPs (or VPN)
- Sentry nodes are disposable and horizontally scalable
- Attacker cannot discover validator IPs

### Rate Limiting

```bash
# Configure rate limiting on the load balancer (nginx/haproxy)
# nginx example:
limit_req_zone $binary_remote_addr zone=rpc:10m rate=50r/s;

server {
  listen 443 ssl;
  server_name rpc.viri.network;

  location / {
    limit_req zone=rpc burst=100 nodelay;
    proxy_pass http://sentry-nodes:8545;
  }
}
```

### Connection Limits

```bash
# Kernel-level tuning
sudo sysctl -w net.core.somaxconn=65535
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sudo sysctl -w net.ipv4.tcp_syncookies=1

# iptables connection limiting
sudo iptables -A INPUT -p tcp --dport 30303 -m connlimit --connlimit-above 100 -j DROP
```

---

## 7. Monitoring Stack Setup

See [MONITORING.md](MONITORING.md) for full details.

```bash
# Quick setup:
cd monitoring
cp prometheus.yml /etc/prometheus/prometheus.yml
cp alerting_rules.yml /etc/prometheus/rules/viri_alerting_rules.yml
cp nodes.yml /etc/prometheus/nodes.yml

# Import Grafana dashboard:
# monitoring/grafana_dashboard.json -> Grafana Dashboard UI
```

---

## 8. Firewall Rules Reference

### Validator Node

```bash
# Deny all incoming by default
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH (from specific IPs only)
sudo ufw allow from <admin-ip> to any port 22 proto tcp

# Allow P2P from sentry nodes only
sudo ufw allow from <sentry-1-private-ip> to any port 30303 proto tcp
sudo ufw allow from <sentry-2-private-ip> to any port 30303 proto tcp
sudo ufw allow from <sentry-3-private-ip> to any port 30303 proto tcp

# Block everything else
sudo ufw deny 8545/tcp
sudo ufw deny 8546/tcp

sudo ufw enable
```

### Sentry Node

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH from admin IPs
sudo ufw allow from <admin-ip> to any port 22 proto tcp

# Open P2P to public
sudo ufw allow 30303/tcp

# Block RPC/API from public
sudo ufw deny 8545/tcp
sudo ufw deny 8546/tcp

# Allow monitoring access from monitoring node
sudo ufw allow from <monitoring-ip> to any port 9100 proto tcp  # node_exporter

sudo ufw enable
```

### Seed/Bootstrap Node

```bash
sudo ufw allow 30303/tcp
sudo ufw allow from <monitoring-ip> to any port 9100 proto tcp
sudo ufw allow from <admin-ip> to any port 22 proto tcp
sudo ufw deny 8545/tcp
sudo ufw deny 8546/tcp
sudo ufw enable
```

---

## 9. DNS Setup

### Public Endpoints

| Record | Type | Value | TTL |
|--------|------|-------|-----|
| `rpc.viri.network` | A | Load balancer IP | 60 |
| `api.viri.network` | A | Load balancer IP | 60 |
| `ws.viri.network` | A | Load balancer IP | 60 |
| `seeds.viri.network` | A | Seed node IP | 300 |

### Seed Node DNS Resolution

```bash
# Configure seed nodes with DNS seed syntax:
# /dns/seeds.viri.network/tcp/30303/p2p/<peer-id>

# In node config:
"bootstrap_peers": [
  "/dns/seeds.viri.network/tcp/30303/p2p/16Uiu2HAm...",
  "/dns/sentry-01.viri.network/tcp/30303/p2p/16Uiu2HAm..."
]
```

---

## 10. Security Checklist

### Pre-Deployment
- [ ] All infrastructure provisioned and tested
- [ ] Sentry nodes deployed in different regions / availability zones
- [ ] Validator key generated on air-gapped machine
- [ ] Validator HSM configured and tested
- [ ] TLS certificates installed
- [ ] API key hash set
- [ ] Firewall rules verified
- [ ] Systemd services installed and tested
- [ ] Daily backup timer enabled
- [ ] Monitoring and alerting configured

### Post-Deployment
- [ ] Verify peer connectivity between validator and sentry nodes
- [ ] Verify validator is signing blocks
- [ ] Verify monitoring dashboards show correct data
- [ ] Verify alerts fire correctly (test by stopping node)
- [ ] Verify backup was created and is restorable
- [ ] Node running as unprivileged `viri` user
- [ ] `NoNewPrivileges=yes` in systemd service
- [ ] `ProtectHome=true`, `ProtectSystem=full` in systemd service

### Weekly
- [ ] Review audit logs
- [ ] Check disk usage
- [ ] Verify backup integrity
- [ ] Check for software updates

### Monthly
- [ ] Review monitoring dashboards for trends
- [ ] Test backup restore on staging
- [ ] Rotate API keys
- [ ] Review and update firewall rules

---

## 11. Load Balancer Configuration

### HAProxy (RPC Endpoints)

```haproxy
global
  maxconn 10000

defaults
  mode http
  timeout connect 5s
  timeout client 30s
  timeout server 30s
  option httpchk GET /health

frontend rpc_frontend
  bind *:443 ssl crt /etc/ssl/viri/
  default_backend rpc_backend

backend rpc_backend
  balance source
  server sentry-01 <sentry-1-ip>:8545 check
  server sentry-02 <sentry-2-ip>:8545 check
  server sentry-03 <sentry-3-ip>:8545 check
```

### Nginx (API Endpoints)

```nginx
upstream api_backend {
  least_conn;
  server <sentry-1-ip>:8546;
  server <sentry-2-ip>:8546;
  server <sentry-3-ip>:8546;
}

server {
  listen 443 ssl;
  server_name api.viri.network;

  ssl_certificate /etc/letsencrypt/live/api.viri.network/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/api.viri.network/privkey.pem;

  location / {
    limit_req zone=api burst=200 nodelay;
    proxy_pass http://api_backend;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
  }
}
```

---

## 12. Emergency Contact Procedures

### Incident Reporting

```bash
# If you detect an anomaly:
# 1. Note the current block height and time
# 2. Collect relevant logs
journalctl -u virid -n 500 --no-pager > /tmp/incident-$(date +%Y%m%d-%H%M%S).log

# 3. Contact the validator coordinator immediately
#    via the emergency communication channel

# 4. DO NOT restart the node unless instructed
#    (restart may destroy forensic evidence)

# 5. If instructed to halt:
sudo systemctl stop virid
sudo cp -r /var/lib/viri /var/lib/viri.incident-$(date +%Y%m%d-%H%M%S)
```

### Emergency Contacts

| Role | Name | Contact |
|------|------|---------|
| Validator Coordinator | <!-- TODO --> | Signal / Keybase |
| Core Dev On-Call | <!-- TODO --> | PagerDuty escalation |
| Infrastructure Lead | <!-- TODO --> | Phone / Signal |
| Security Lead | <!-- TODO --> | Encrypted email |

**Fill in contact details before mainnet launch and distribute to all validators via secure channel.**
