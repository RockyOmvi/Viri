#!/usr/bin/env bash
# Viri Public Testnet — Standalone Provisioning Script
# No dependencies: runs with Docker on any Linux x86_64/ARM64 VM.
#
# Usage:
#   sudo ./standalone-provision.sh <role> [options]
#
# Roles:
#   bootstrap     Bootstrap node (generates node key, seeds P2P mesh)
#   validator     Validator node (requires --validator-key and --index)
#   faucet        Faucet + explorer node (requires --faucet-key)
#
# Options:
#   --genesis PATH      Path to genesis.json (required for validators, faucet)
#   --config PATH       Path to node config.json (optional)
#   --validator-key PATH Path to validator private key file (required for validator)
#   --index N           Validator index 0-3 (required for validator)
#   --faucet-key HEX    Faucet wallet private key hex (required for faucet)
#   --bootstrap-addr ADDR Bootstrap node public IP (required for validators/faucet)
#   --bootstrap-peer-id ID Bootstrap peer ID (required for validators/faucet)
#   --chain-id N        Chain ID (default: 2)
#   --domain DOMAIN     Domain name for TLS (optional)
#   --pull              Force pull latest Docker image
#   --help              Show this help

set -euo pipefail

ROLE=""
GENESIS_PATH=""
CONFIG_PATH=""
VALIDATOR_KEY_PATH=""
VALIDATOR_INDEX=""
FAUCET_KEY=""
BOOTSTRAP_ADDR=""
BOOTSTRAP_PEER_ID=""
CHAIN_ID=2
DOMAIN=""
FORCE_PULL=false

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { echo -e "${BLUE}[provision]${NC} $1"; }
ok()   { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

usage() {
  echo "Viri Testnet — Standalone Node Provisioner"
  echo ""
  echo "Usage: sudo $0 <role> [options]"
  echo ""
  echo "Roles:"
  echo "  bootstrap     Bootstrap/seeder node"
  echo "  validator     Consensus validator (use --index for 0-3)"
  echo "  faucet        Faucet + block explorer"
  echo ""
  echo "Options:"
  echo "  --genesis PATH         Genesis JSON file (required for validators and faucet)"
  echo "  --config PATH          Node config JSON (optional, auto-generated)"
  echo "  --validator-key PATH   Validator private key file (required for validator)"
  echo "  --index N              Validator index 0-3 (required for validator)"
  echo "  --faucet-key HEX       Faucet wallet private key (required for faucet)"
  echo "  --bootstrap-addr ADDR  Bootstrap node public IP"
  echo "  --bootstrap-peer-id ID Bootstrap peer ID"
  echo "  --chain-id N           Chain ID (default: 2)"
  echo "  --domain DOMAIN        TLS domain (optional)"
  echo "  --pull                 Force pull latest Docker image"
  echo "  --help                 Show this help"
  exit 0
}

# --- Parse args ---
ROLE="${1:-}"; shift || true
if [[ -z "$ROLE" || "$ROLE" == "--help" ]]; then usage; fi

case "$ROLE" in
  bootstrap|validator|faucet) ;;
  *) fail "Unknown role: $ROLE. Use bootstrap, validator, or faucet." ;;
esac

while [[ $# -gt 0 ]]; do
  case $1 in
    --genesis)         GENESIS_PATH="$2"; shift 2 ;;
    --config)          CONFIG_PATH="$2"; shift 2 ;;
    --validator-key)   VALIDATOR_KEY_PATH="$2"; shift 2 ;;
    --index)           VALIDATOR_INDEX="$2"; shift 2 ;;
    --faucet-key)      FAUCET_KEY="$2"; shift 2 ;;
    --bootstrap-addr)  BOOTSTRAP_ADDR="$2"; shift 2 ;;
    --bootstrap-peer-id) BOOTSTRAP_PEER_ID="$2"; shift 2 ;;
    --chain-id)        CHAIN_ID="$2"; shift 2 ;;
    --domain)          DOMAIN="$2"; shift 2 ;;
    --pull)            FORCE_PULL=true; shift ;;
    --help)            usage ;;
    *) fail "Unknown option: $1" ;;
  esac
done

# --- Validate ---
if [[ "$ROLE" == "validator" ]]; then
  [[ -z "$VALIDATOR_KEY_PATH" ]] && fail "validator requires --validator-key"
  [[ -z "$VALIDATOR_INDEX" ]] && fail "validator requires --index"
  [[ -z "$GENESIS_PATH" ]] && fail "validator requires --genesis"
  [[ -z "$BOOTSTRAP_ADDR" ]] && fail "validator requires --bootstrap-addr"
  [[ -z "$BOOTSTRAP_PEER_ID" ]] && fail "validator requires --bootstrap-peer-id"
fi

if [[ "$ROLE" == "faucet" ]]; then
  [[ -z "$FAUCET_KEY" ]] && fail "faucet requires --faucet-key"
fi

if [[ "$ROLE" == "bootstrap" ]]; then
  if [[ -z "$GENESIS_PATH" ]]; then
    warn "No genesis provided — bootstrap will generate a default one"
  fi
fi

[[ $EUID -ne 0 ]] && fail "This script must be run as root (sudo)"

# =====================================================================
# Step 1: Install Docker if not present
# =====================================================================
log "Checking Docker..."

if ! command -v docker &>/dev/null; then
  log "Installing Docker..."
  curl -fsSL https://get.docker.com | sh
  systemctl enable docker
  systemctl start docker
  ok "Docker installed"
else
  ok "Docker found: $(docker --version)"
fi

if ! command -v docker compose &>/dev/null; then
  log "Installing Docker Compose plugin..."
  DOCKER_CONFIG=${DOCKER_CONFIG:-/usr/local/lib/docker/cli-plugins}
  mkdir -p "$DOCKER_CONFIG"
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)  DC_ARCH="x86_64" ;;
    aarch64) DC_ARCH="aarch64" ;;
    *)       fail "Unsupported arch: $ARCH" ;;
  esac
  curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-${DC_ARCH}" \
    -o "$DOCKER_CONFIG/docker-compose"
  chmod +x "$DOCKER_CONFIG/docker-compose"
  ok "Docker Compose installed"
fi

# =====================================================================
# Step 2: Pull image
# =====================================================================
IMAGE="ghcr.io/viri-chain/viri:latest"
log "Pulling $IMAGE..."
docker pull "$IMAGE"
if $FORCE_PULL; then
  docker pull "$IMAGE"
fi
ok "Image ready"

# =====================================================================
# Step 3: Setup directories
# =====================================================================
log "Setting up directories..."
mkdir -p /opt/viri/{config,data,logs,genesis}
ok "Directories created"

# =====================================================================
# Step 4: Copy genesis
# =====================================================================
if [[ -n "$GENESIS_PATH" ]]; then
  if [[ ! -f "$GENESIS_PATH" ]]; then
    fail "Genesis file not found: $GENESIS_PATH"
  fi
  cp "$GENESIS_PATH" /opt/viri/genesis/genesis.json
  chmod 644 /opt/viri/genesis/genesis.json
  ok "Genesis copied"
fi

# =====================================================================
# Step 5: Copy validator key
# =====================================================================
if [[ "$ROLE" == "validator" && -n "$VALIDATOR_KEY_PATH" ]]; then
  if [[ ! -f "$VALIDATOR_KEY_PATH" ]]; then
    fail "Validator key not found: $VALIDATOR_KEY_PATH"
  fi
  cp "$VALIDATOR_KEY_PATH" /opt/viri/config/validator.key
  chmod 600 /opt/viri/config/validator.key
  ok "Validator key copied"
fi

# =====================================================================
# Step 6: Generate node config
# =====================================================================
if [[ -n "$CONFIG_PATH" ]]; then
  if [[ ! -f "$CONFIG_PATH" ]]; then
    fail "Config not found: $CONFIG_PATH"
  fi
  cp "$CONFIG_PATH" /opt/viri/config/config.json
  ok "Config copied from $CONFIG_PATH"
else
  log "Generating node config..."

  NODE_NAME="${ROLE}"
  VALIDATOR_MODE="false"
  VALIDATOR_KEY=""

  if [[ "$ROLE" == "validator" ]]; then
    NODE_NAME="validator-${VALIDATOR_INDEX}"
    VALIDATOR_MODE="true"
    VALIDATOR_KEY="\/home\/viri\/config\/validator.key"
  fi

  BOOTSTRAP_PEERS="[]"
  if [[ -n "$BOOTSTRAP_ADDR" && -n "$BOOTSTRAP_PEER_ID" ]]; then
    BOOTSTRAP_PEERS="[\"\/dns\/${BOOTSTRAP_ADDR}\/tcp\/30303\/p2p\/${BOOTSTRAP_PEER_ID}\"]"
  fi

  cat > /opt/viri/config/config.json << CONFIGEOF
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
    "external_addr": "",
    "bootstrap_peers": $BOOTSTRAP_PEERS,
    "max_peers": 50,
    "enable_dht": true,
    "enable_nat": true
  },
  "node": {
    "name": "$NODE_NAME",
    "data_dir": "/home/viri/.viri",
    "validator_mode": $VALIDATOR_MODE,
    "validator_key": "$VALIDATOR_KEY",
    "rpc_enabled": true,
    "rpc_port": 8545,
    "api_enabled": true,
    "api_port": 8546,
    "tls_cert_path": "",
    "tls_key_path": "",
    "api_key_hash": ""
  },
  "consensus": {
    "min_stake": 1000000,
    "max_validators": 50,
    "epoch_length": 500,
    "slashing_enabled": true,
    "finality_threshold": "2s"
  },
  "storage": {
    "backend": "leveldb",
    "path": "/home/viri/.viri/chaindata",
    "max_state_size": 5368709120,
    "pruning_enabled": true,
    "pruning_keep_recent": 50000,
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
CONFIGEOF
  ok "Config generated for role: $ROLE"
fi

# =====================================================================
# Step 7: Bootstrap node key generation
# =====================================================================
if [[ "$ROLE" == "bootstrap" ]]; then
  if [[ ! -f /opt/viri/config/bootstrap.key ]]; then
    log "Generating bootstrap node key..."
    docker run --rm -v /opt/viri/config:/config "$IMAGE" \
      virid --generate-node-key --key-file /config/bootstrap.key
    ok "Bootstrap key generated"
  else
    ok "Bootstrap key already exists"
  fi
  BOOTSTRAP_PEER_ID=$(grep -oP 'peer_id:\s*\K\S+' /opt/viri/config/bootstrap.key || true)
  if [[ -n "$BOOTSTRAP_PEER_ID" ]]; then
    echo "$BOOTSTRAP_PEER_ID" > /opt/viri/config/peer_id.txt
    ok "Bootstrap Peer ID: $BOOTSTRAP_PEER_ID"
  fi
fi

# =====================================================================
# Step 8: Generate docker-compose.yml
# =====================================================================
log "Generating docker-compose.yml..."

case "$ROLE" in
  bootstrap)
    cat > /opt/viri/docker-compose.yml << 'COMPOSE'
services:
  node:
    image: ghcr.io/viri-chain/viri:latest
    container_name: viri-bootstrap
    ports:
      - "30303:30303"
      - "8545:8545"
      - "8546:8546"
    volumes:
      - /opt/viri/config:/home/viri/config:ro
      - /opt/viri/genesis:/home/viri/data:ro
      - /opt/viri/data:/home/viri/.viri
    environment:
      - VIRI_NODE_NAME=bootstrap-0
      - VIRI_LOG_LEVEL=info
      - VIRI_TESTNET=true
      - VIRI_RPC_PORT=8545
      - VIRI_API_PORT=8546
    restart: unless-stopped
    command: ["virid", "--config", "/home/viri/config/config.json"]
COMPOSE
    ;;

  validator)
    cat > /opt/viri/docker-compose.yml << COMPOSE
services:
  node:
    image: ghcr.io/viri-chain/viri:latest
    container_name: viri-validator-${VALIDATOR_INDEX}
    ports:
      - "30303:30303"
      - "8545:8545"
      - "8546:8546"
      - "9090:9090"
    volumes:
      - /opt/viri/config:/home/viri/config:ro
      - /opt/viri/genesis:/home/viri/data:ro
      - /opt/viri/data:/home/viri/.viri
    environment:
      - VIRI_NODE_NAME=validator-${VALIDATOR_INDEX}
      - VIRI_LOG_LEVEL=info
      - VIRI_TESTNET=true
      - VIRI_VALIDATOR_MODE=true
      - VIRI_VALIDATOR_KEY=/home/viri/config/validator.key
      - VIRI_RPC_PORT=8545
      - VIRI_API_PORT=8546
      - VIRI_CONSENSUS_DELAY=60s
    restart: unless-stopped
    command: ["virid", "--config", "/home/viri/config/config.json"]
COMPOSE
    ;;

  faucet)
    cat > /opt/viri/docker-compose.yml << COMPOSE
services:
  faucet:
    image: ghcr.io/viri-chain/viri:latest
    container_name: viri-faucet
    ports:
      - "8081:8081"
    environment:
      - VIRI_LOG_LEVEL=info
      - FAUCET_PORT=8081
      - FAUCET_WALLET_KEY=${FAUCET_KEY}
      - FAUCET_DAILY_LIMIT=1000000000000000000000
      - FAUCET_PER_CLAIM=10000000000000000000
      - VIRI_RPC_URL=http://${BOOTSTRAP_ADDR}:8545
    restart: unless-stopped
    command: ["virid", "--faucet"]

  explorer:
    image: ghcr.io/viri-chain/viri:latest
    container_name: viri-explorer
    ports:
      - "8080:8080"
    environment:
      - EXPLORER_PORT=8080
      - VIRI_RPC_URL=http://${BOOTSTRAP_ADDR}:8545
    restart: unless-stopped
    command: ["virid", "--explorer"]
COMPOSE
    ;;
esac

ok "docker-compose.yml generated"

# =====================================================================
# Step 9: Start services
# =====================================================================
log "Starting services..."
cd /opt/viri
docker compose up -d
ok "Services started for role: $ROLE"

# =====================================================================
# Step 10: Print info
# =====================================================================
echo ""
echo "============================================"
echo "  Viri Node — Provisioning Complete"
echo "============================================"
echo "  Role:      $ROLE"
echo "  Chain ID:  $CHAIN_ID"

case "$ROLE" in
  bootstrap)
    PEER=$(cat /opt/viri/config/peer_id.txt 2>/dev/null || echo "unknown")
    echo "  Peer ID:   $PEER"
    echo "  RPC:       http://$(curl -s ifconfig.me):8545"
    echo "  API:       http://$(curl -s ifconfig.me):8546"
    ;;
  validator)
    echo "  Index:     $VALIDATOR_INDEX"
    echo "  RPC:       http://$(curl -s ifconfig.me):8545"
    ;;
  faucet)
    echo "  Faucet:    http://$(curl -s ifconfig.me):8081"
    echo "  Explorer:  http://$(curl -s ifconfig.me):8080"
    ;;
esac

echo ""
echo "  Logs:      docker compose -f /opt/viri/docker-compose.yml logs -f"
echo "  Stop:      docker compose -f /opt/viri/docker-compose.yml down"
echo "============================================"
