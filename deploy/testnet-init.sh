#!/usr/bin/env bash
# Viri Blockchain - Testnet Deployment Script
# Automated deployment for multi-validator testnet with monitoring
#
# Usage: ./testnet-init.sh [OPTIONS]
#   --validators N       Number of validators (default: 4)
#   --chain-id ID        Chain ID (default: 1337)
#   --stake AMOUNT       Initial stake per validator (default: 1000000)
#   --output-dir DIR     Output directory (default: ./testnet)
#   --monitoring         Enable Prometheus/Grafana (default: false)
#   --explorer           Enable block explorer (default: false)
#   --faucet             Enable faucet (default: false)
#   --docker             Use Docker Compose (default: true)
#   --build              Build binaries first (default: false)
#   --help               Show this help message

set -euo pipefail

# Default values
VALIDATORS=4
CHAIN_ID=1337
STAKE=1000000
OUTPUT_DIR="./testnet"
MONITORING=false
EXPLORER=false
FAUCET=false
USE_DOCKER=true
BUILD_FIRST=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()    { echo -e "\n${CYAN}==> $1${NC}"; }

usage() {
    echo -e "${BLUE}Viri Blockchain - Testnet Deployment Script${NC}"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --validators N       Number of validators (default: 4)"
    echo "  --chain-id ID        Chain ID (default: 1337)"
    echo "  --stake AMOUNT       Initial stake per validator (default: 1000000)"
    echo "  --output-dir DIR     Output directory (default: ./testnet)"
    echo "  --monitoring         Enable Prometheus/Grafana"
    echo "  --explorer           Enable block explorer"
    echo "  --faucet             Enable faucet"
    echo "  --docker             Use Docker Compose (default: true)"
    echo "  --build              Build binaries first"
    echo "  --help               Show this help"
    echo ""
    echo "Examples:"
    echo "  # Quick start with 4 validators"
    echo "  $0"
    echo ""
    echo "  # Full testnet with monitoring"
    echo "  $0 --validators 4 --monitoring --explorer --faucet"
    echo ""
    echo "  # Custom chain ID with 6 validators"
    echo "  $0 --validators 6 --chain-id 9999 --stake 5000000"
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --validators) VALIDATORS="$2"; shift 2 ;;
        --chain-id)   CHAIN_ID="$2"; shift 2 ;;
        --stake)      STAKE="$2"; shift 2 ;;
        --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
        --monitoring) MONITORING=true; shift ;;
        --explorer)   EXPLORER=true; shift ;;
        --faucet)     FAUCET=true; shift ;;
        --docker)     USE_DOCKER=true; shift ;;
        --build)      BUILD_FIRST=true; shift ;;
        --help|-h)    usage ;;
        *)            log_error "Unknown option: $1"; usage ;;
    esac
done

# Validate
if [[ $VALIDATORS -lt 1 ]]; then
    log_error "Minimum 1 validator required"
    exit 1
fi

if [[ $VALIDATORS -lt 3 ]]; then
    log_warn "HotStuff BFT recommends minimum 3 validators for fault tolerance"
fi

if ! command -v go &> /dev/null && [[ "$BUILD_FIRST" == "true" ]]; then
    log_error "Go is required for building. Install from https://golang.org/dl/"
    exit 1
fi

log_step "Testnet Configuration"
echo "  Validators: $VALIDATORS"
echo "  Chain ID:   $CHAIN_ID"
echo "  Stake/Node: $STAKE"
echo "  Output:     $OUTPUT_DIR"
echo "  Monitoring: $MONITORING"
echo "  Explorer:   $EXPLORER"
echo "  Faucet:     $FAUCET"
echo ""

# Build binaries if requested
if [[ "$BUILD_FIRST" == "true" ]]; then
    log_step "Building binaries..."
    cd "$(dirname "$0")/.."
    make build 2>/dev/null || go build -o ./virid ./cmd/virid && go build -o ./virictl ./cmd/virictl
    log_success "Binaries built"
fi

# Clean output directory
if [[ -d "$OUTPUT_DIR" ]]; then
    log_warn "Output directory exists. Cleaning..."
    rm -rf "$OUTPUT_DIR"
fi

mkdir -p "$OUTPUT_DIR"/{genesis,keys,configs}

log_step "Generating validator keys..."

# Generate validator keys using Go
VALIDATOR_ADDRESSES=()
VALIDATOR_PUBKEYS=()
VALIDATOR_KEYS=()

KEYGEN_SCRIPT=$(mktemp)
cat > "$KEYGEN_SCRIPT" << 'GOEOF'
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate key: %v\n", err)
		os.Exit(1)
	}

	pubKey := privKey.PublicKey
	pubKeyBytes := elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)

	hash := sha256.Sum256(pubKeyBytes)
	address := hex.EncodeToString(hash[:20])

	privKeyBytes := privKey.D.Bytes()

	fmt.Printf("%s %s %s\n",
		hex.EncodeToString(privKeyBytes),
		hex.EncodeToString(pubKeyBytes),
		address)
}
GOEOF

for i in $(seq 0 $((VALIDATORS - 1))); do
    KEY_OUTPUT=$(go run "$KEYGEN_SCRIPT" 2>/dev/null)
    PRIV_KEY=$(echo "$KEY_OUTPUT" | awk '{print $1}')
    PUB_KEY=$(echo "$KEY_OUTPUT" | awk '{print $2}')
    ADDRESS=$(echo "$KEY_OUTPUT" | awk '{print $3}')

    VALIDATOR_KEYS+=("$PRIV_KEY")
    VALIDATOR_PUBKEYS+=("$PUB_KEY")
    VALIDATOR_ADDRESSES+=("0x$ADDRESS")

    # Save key files
    cat > "$OUTPUT_DIR/keys/validator-$i.json" << EOF
{
    "address": "0x$ADDRESS",
    "public_key": "$PUB_KEY",
    "stake": $STAKE,
    "name": "validator-$i"
}
EOF
    echo "$PRIV_KEY" > "$OUTPUT_DIR/keys/validator-$i.key"
    chmod 600 "$OUTPUT_DIR/keys/validator-$i.key"

    log_success "Validator $i: 0x${ADDRESS:0:16}..."
done

rm -f "$KEYGEN_SCRIPT"

log_step "Creating genesis configuration..."

GENESIS_VALIDATORS="["
for i in $(seq 0 $((VALIDATORS - 1))); do
    [[ $i -gt 0 ]] && GENESIS_VALIDATORS+=","
    GENESIS_VALIDATORS+="
    {
      \"address\": \"${VALIDATOR_ADDRESSES[$i]}\",
      \"public_key\": \"${VALIDATOR_PUBKEYS[$i]}\",
      \"stake\": $STAKE,
      \"name\": \"validator-$i\"
    }"
done
GENESIS_VALIDATORS+="
  ]"

TOTAL_STAKE=$((STAKE * VALIDATORS))
GENESIS_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

cat > "$OUTPUT_DIR/genesis/genesis.json" << EOF
{
  "chain_id": $CHAIN_ID,
  "network_name": "viri-testnet",
  "genesis_time": "$GENESIS_TIME",
  "hash": "",
  "validators": $GENESIS_VALIDATORS,
  "total_stake": $TOTAL_STAKE,
  "version": "0.1.0"
}
EOF

log_success "Genesis created (total stake: $TOTAL_STAKE)"

log_step "Generating node configurations..."

# Generate Docker Compose
COMPOSE_FILE="$OUTPUT_DIR/docker-compose.yml"

cat > "$COMPOSE_FILE" << EOF
version: "3.8"

services:
EOF

# Add validator services
for i in $(seq 0 $((VALIDATORS - 1))); do
    P2P_PORT=$((30303 + i * 10))
    RPC_PORT=$((8545 + i))
    API_PORT=$((8546 + i))

    # Build bootstrap peers list (all other validators)
    BOOTSTRAP_PEERS=""
    for j in $(seq 0 $((VALIDATORS - 1))); do
        if [[ $i -ne $j ]]; then
            PEER_PORT=$((30303 + j * 10))
            BOOTSTRAP_PEERS+="- /ip4/host.docker.internal/tcp/$PEER_PORT/p2p/placeholder-peer-$j\n"
        fi
    done

    cat >> "$COMPOSE_FILE" << EOF
  validator-$i:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: viri-validator-$i
    ports:
      - "$P2P_PORT:30303"
      - "$RPC_PORT:8545"
      - "$API_PORT:8546"
    volumes:
      - ./configs/validator-$i:/home/viri/config:ro
      - ./genesis/genesis.json:/home/viri/data/genesis.json:ro
      - validator-$i-data:/home/viri/.viri
    environment:
      - VIRI_NODE_NAME=validator-$i
      - VIRI_DATA_DIR=/home/viri/.viri
      - VIRI_RPC_PORT=8545
      - VIRI_API_PORT=8546
      - VIRI_LOG_LEVEL=info
      - VIRI_VALIDATOR_MODE=true
      - VIRI_VALIDATOR_KEY=/keys/validator.key
      - VIRI_CHAIN_ID=$CHAIN_ID
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9090/health"]
      interval: 10s
      timeout: 5s
      retries: 3

EOF
done

# Add monitoring services if enabled
if [[ "$MONITORING" == "true" ]]; then
    cat >> "$COMPOSE_FILE" << EOF
  prometheus:
    image: prom/prometheus:latest
    container_name: viri-prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./configs/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    restart: unless-stopped
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--storage.tsdb.path=/prometheus"
      - "--web.enable-lifecycle"

  grafana:
    image: grafana/grafana:latest
    container_name: viri-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana:/etc/grafana/provisioning:ro
    restart: unless-stopped
    depends_on:
      - prometheus

EOF
fi

# Add explorer if enabled
if [[ "$EXPLORER" == "true" ]]; then
    cat >> "$COMPOSE_FILE" << EOF
  explorer:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: viri-explorer
    ports:
      - "8080:8080"
    environment:
      - EXPLORER_PORT=8080
      - VIRI_RPC_URL=http://validator-0:8545
    restart: unless-stopped
    depends_on:
      - validator-0
    command: ["virid", "--explorer"]

EOF
fi

# Add faucet if enabled
if [[ "$FAUCET" == "true" ]]; then
    cat >> "$COMPOSE_FILE" << EOF
  faucet:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: viri-faucet
    ports:
      - "8081:8081"
    environment:
      - FAUCET_PORT=8081
      - FAUCET_WALLET_KEY=\${FAUCET_WALLET_KEY:-}
      - FAUCET_DAILY_LIMIT=1000000000000000000000
      - FAUCET_PER_CLAIM=10000000000000000000
    restart: unless-stopped
    depends_on:
      - validator-0
    command: ["virid", "--faucet"]

EOF
fi

# Add volumes
cat >> "$COMPOSE_FILE" << EOF
volumes:
EOF
for i in $(seq 0 $((VALIDATORS - 1))); do
    echo "  validator-$i-data:" >> "$COMPOSE_FILE"
done
if [[ "$MONITORING" == "true" ]]; then
    echo "  prometheus-data:" >> "$COMPOSE_FILE"
    echo "  grafana-data:" >> "$COMPOSE_FILE"
fi

log_success "Docker Compose generated"

# Generate per-validator configs
for i in $(seq 0 $((VALIDATORS - 1))); do
    P2P_PORT=$((30303 + i * 10))
    RPC_PORT=8545
    API_PORT=8546

    # Build bootstrap peers (all other validators)
    BOOTSTRAP_PEERS="["
    FIRST=true
    for j in $(seq 0 $((VALIDATORS - 1))); do
        if [[ $i -ne $j ]]; then
            PEER_PORT=$((30303 + j * 10))
            [[ "$FIRST" != "true" ]] && BOOTSTRAP_PEERS+=","
            BOOTSTRAP_PEERS+="\"/dns/validator-$j/tcp/30303/p2p/placeholder\""
            FIRST=false
        fi
    done
    BOOTSTRAP_PEERS+="]"

    mkdir -p "$OUTPUT_DIR/configs/validator-$i"

    cat > "$OUTPUT_DIR/configs/validator-$i/config.json" << EOF
{
  "chain": {
    "chain_id": $CHAIN_ID,
    "network_name": "viri-testnet",
    "block_time": "1s",
    "max_block_size": 10485760,
    "max_gas_per_block": 30000000,
    "genesis_file": "/home/viri/data/genesis.json"
  },
  "network": {
    "listen_addr": "0.0.0.0:30303",
    "bootstrap_peers": $BOOTSTRAP_PEERS,
    "max_peers": 50,
    "enable_dht": true,
    "enable_nat": false
  },
  "node": {
    "name": "validator-$i",
    "data_dir": "/home/viri/.viri",
    "validator_mode": true,
    "validator_key": "/keys/validator.key",
    "rpc_enabled": true,
    "rpc_port": $RPC_PORT,
    "api_enabled": true,
    "api_port": $API_PORT,
    "metrics_enabled": true,
    "metrics_port": 9090
  },
  "consensus": {
    "min_stake": 10000,
    "max_validators": $VALIDATORS,
    "epoch_length": 100,
    "slashing_enabled": true,
    "finality_threshold": "2s"
  },
  "storage": {
    "backend": "leveldb",
    "path": "/home/viri/.viri/chaindata",
    "max_state_size": 10737418240,
    "pruning_enabled": true,
    "pruning_keep_recent": 100000,
    "archive_mode": false
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout",
    "max_size": 100,
    "max_backups": 3
  },
  "readiness": {
    "min_peers": 1,
    "min_block_height": 0,
    "force_ready": false
  }
}
EOF

    # Copy validator key
    echo "${VALIDATOR_KEYS[$i]}" > "$OUTPUT_DIR/configs/validator-$i/validator.key"
done

log_success "Node configurations created"

# Generate monitoring configs if enabled
if [[ "$MONITORING" == "true" ]]; then
    mkdir -p "$OUTPUT_DIR/configs/prometheus" "$OUTPUT_DIR/grafana/datasources" "$OUTPUT_DIR/grafana/dashboards"

    # Build Prometheus targets
    PROM_TARGETS=""
    for i in $(seq 0 $((VALIDATORS - 1))); do
        [[ $i -gt 0 ]] && PROM_TARGETS+=","
        PROM_TARGETS+="\"validator-$i:9090\""
    done

    cat > "$OUTPUT_DIR/configs/prometheus.yml" << EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'viri-validators'
    static_configs:
      - targets: [$PROM_TARGETS]
        labels:
          network: 'viri-testnet'

  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
EOF

    cat > "$OUTPUT_DIR/grafana/datasources/datasources.yml" << EOF
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
EOF

    cat > "$OUTPUT_DIR/grafana/dashboards/dashboard-provider.yml" << EOF
apiVersion: 1
providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    options:
      path: /etc/grafana/provisioning/dashboards
EOF

    cat > "$OUTPUT_DIR/grafana/dashboards/overview.json" << 'GRAFANAEOF'
{
  "annotations": {"list": []},
  "editable": true,
  "fiscalYearStartMonth": 0,
  "graphTooltip": 0,
  "links": [],
  "liveNow": false,
  "panels": [
    {
      "title": "Block Height",
      "type": "stat",
      "targets": [
        {"expr": "viri_block_height", "legendFormat": "{{instance}}"}
      ],
      "fieldConfig": {"defaults": {"color": {"mode": "palette-classic"}}},
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0}
    },
    {
      "title": "Peer Count",
      "type": "stat",
      "targets": [
        {"expr": "viri_peer_count", "legendFormat": "{{instance}}"}
      ],
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0}
    },
    {
      "title": "Consensus Phase",
      "type": "stat",
      "targets": [
        {"expr": "viri_consensus_phase", "legendFormat": "{{instance}}"}
      ],
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8}
    },
    {
      "title": "Blocks Finalized",
      "type": "stat",
      "targets": [
        {"expr": "viri_blocks_finalized_total", "legendFormat": "{{instance}}"}
      ],
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8}
    }
  ],
  "schemaVersion": 38,
  "style": "dark",
  "tags": ["viri", "blockchain"],
  "templating": {"list": []},
  "time": {"from": "now-1h", "to": "now"},
  "title": "Viri Testnet Overview",
  "uid": "viri-testnet",
  "version": 1
}
GRAFANAEOF
fi

log_step "Generating deployment scripts..."

# Create start script
cat > "$OUTPUT_DIR/start.sh" << 'EOF'
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if ! command -v docker &> /dev/null; then
    echo "Docker is required. Install from https://docs.docker.com/get-docker/"
    exit 1
fi

echo "Starting Viri Testnet..."
docker compose up -d

echo ""
echo "Waiting for validators to initialize..."
sleep 10

echo "Checking validator status..."
for i in $(seq 0 $((VALIDATORS - 1))); do
    RPC_PORT=$((8545 + i))
    STATUS=$(curl -sf http://localhost:$RPC_PORT/health 2>/dev/null || echo "not ready")
    echo "  Validator $i: $STATUS"
done

echo ""
echo "Testnet started successfully!"
echo "View logs: docker compose logs -f"
echo "Stop: docker compose down"
EOF
chmod +x "$OUTPUT_DIR/start.sh"

# Create stop script
cat > "$OUTPUT_DIR/stop.sh" << 'EOF'
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "Stopping Viri Testnet..."
docker compose down

echo "Done. Data preserved in Docker volumes."
echo "To remove all data: docker compose down -v"
EOF
chmod +x "$OUTPUT_DIR/stop.sh"

# Create status script
cat > "$OUTPUT_DIR/status.sh" << 'STATUSEOF'
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "=== Viri Testnet Status ==="
echo ""

# Container status
echo "Containers:"
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "  Not running"
echo ""

# Validator status
echo "Validators:"
for i in $(seq 0 $((VALIDATORS - 1))); do
    RPC_PORT=$((8545 + i))
    HEIGHT=$(curl -sf http://localhost:$RPC_PORT/health 2>/dev/null | grep -o '"height":[0-9]*' | cut -d: -f2 || echo "N/A")
    PEERS=$(curl -sf http://localhost:$RPC_PORT/health 2>/dev/null | grep -o '"peers":[0-9]*' | cut -d: -f2 || echo "N/A")
    echo "  Validator $i (RPC:$RPC_PORT) - Height: $HEIGHT, Peers: $PEERS"
done
echo ""

# Block explorer
if curl -sf http://localhost:8080 > /dev/null 2>&1; then
    echo "Explorer: http://localhost:8080"
fi

# Faucet
if curl -sf http://localhost:8081 > /dev/null 2>&1; then
    echo "Faucet:   http://localhost:8081"
fi

# Monitoring
if curl -sf http://localhost:9091 > /dev/null 2>&1; then
    echo "Prometheus: http://localhost:9091"
fi
if curl -sf http://localhost:3000 > /dev/null 2>&1; then
    echo "Grafana:    http://localhost:3000 (admin/admin)"
fi
STATUSEOF
chmod +x "$OUTPUT_DIR/status.sh"

# Make README for the testnet
cat > "$OUTPUT_DIR/README.md" << EOF
# Viri Testnet

Chain ID: $CHAIN_ID
Validators: $VALIDATORS
Network: viri-testnet

## Quick Start

\`\`\`bash
# Start the network
./start.sh

# Check status
./status.sh

# Stop the network
./stop.sh
\`\`\`

## Validator Endpoints

$(for i in $(seq 0 $((VALIDATORS - 1))); do
    RPC_PORT=$((8545 + i))
    API_PORT=$((8546 + i))
    echo "- Validator $i: RPC http://localhost:$RPC_PORT, API http://localhost:$API_PORT"
done)

$(if [[ "$MONITORING" == "true" ]]; then
    echo "## Monitoring
- Prometheus: http://localhost:9091
- Grafana: http://localhost:3000 (admin/admin)"
fi)

$(if [[ "$EXPLORER" == "true" ]]; then
    echo "## Block Explorer
- Explorer: http://localhost:8080"
fi)

$(if [[ "$FAUCET" == "true" ]]; then
    echo "## Faucet
- Faucet: http://localhost:8081
- Daily limit: 1000 VIRI per address"
fi)

## Using virictl

\`\`\`bash
# Check status
./virictl status --rpc http://localhost:8545

# Get latest block
./virictl block latest --rpc http://localhost:8545

# Check balance
./virictl account balance <address> --rpc http://localhost:8545
\`\`\`

## Configuration
- Genesis: \`genesis/genesis.json\`
- Validator keys: \`keys/\`
- Node configs: \`configs/validator-N/\`
EOF

log_success "Deployment scripts created"

# Generate summary
log_step "Deployment Summary"

cat > "$OUTPUT_DIR/SUMMARY.txt" << EOF
========================================
  Viri Blockchain Testnet - Summary
========================================

Chain ID:     $CHAIN_ID
Network:      viri-testnet
Validators:   $VALIDATORS
Total Stake:  $TOTAL_STAKE
Genesis Time: $GENESIS_TIME

----------------------------------------
  Validator Information
----------------------------------------
$(for i in $(seq 0 $((VALIDATORS - 1))); do
    P2P_PORT=$((30303 + i * 10))
    RPC_PORT=$((8545 + i))
    API_PORT=$((8546 + i))
    echo "
Validator $i:
  Address:  ${VALIDATOR_ADDRESSES[$i]}
  P2P Port: $P2P_PORT
  RPC Port: $RPC_PORT (http://localhost:$RPC_PORT)
  API Port: $API_PORT (http://localhost:$API_PORT)
  Key File: keys/validator-$i.key
  Config:   configs/validator-$i/config.json"
done)

----------------------------------------
  Quick Start
----------------------------------------

  cd $OUTPUT_DIR
  ./start.sh       # Start all nodes
  ./status.sh      # Check status
  ./stop.sh        # Stop all nodes

----------------------------------------
  Services
----------------------------------------
$(if [[ "$MONITORING" == "true" ]]; then
    echo "  Prometheus:   http://localhost:9091"
    echo "  Grafana:      http://localhost:3000 (admin/admin)"
fi)
$(if [[ "$EXPLORER" == "true" ]]; then
    echo "  Explorer:     http://localhost:8080"
fi)
$(if [[ "$FAUCET" == "true" ]]; then
    echo "  Faucet:       http://localhost:8081"
fi)

========================================
EOF

echo ""
cat "$OUTPUT_DIR/SUMMARY.txt"

log_success "Testnet deployment ready at: $OUTPUT_DIR"
log_info "Run 'cd $OUTPUT_DIR && ./start.sh' to launch"
