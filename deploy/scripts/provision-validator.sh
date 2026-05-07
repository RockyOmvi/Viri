#!/usr/bin/env bash
# Viri Blockchain - Validator Node Provisioning Script
# Installs and configures a validator node on a fresh Linux server
#
# Usage: sudo ./provision-validator.sh [OPTIONS]
#   --key-file PATH     Path to validator private key file
#   --config PATH       Path to node config file
#   --peer ADDRS        Bootstrap peer multiaddress (can be repeated)
#   --chain-id ID       Chain ID (default: 1337)
#   --name NAME         Validator name (default: validator)
#   --help              Show this help

set -euo pipefail

KEY_FILE=""
CONFIG_FILE=""
PEERS=()
CHAIN_ID=1337
NAME="validator"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
    echo "Viri Blockchain - Validator Node Provisioning"
    echo ""
    echo "Usage: sudo $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --key-file PATH     Path to validator private key"
    echo "  --config PATH       Path to node config (optional, will generate)"
    echo "  --peer ADDRS        Bootstrap peer (can repeat)"
    echo "  --chain-id ID       Chain ID (default: 1337)"
    echo "  --name NAME         Validator name"
    echo "  --help              Show this help"
    exit 0
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --key-file) KEY_FILE="$2"; shift 2 ;;
        --config)   CONFIG_FILE="$2"; shift 2 ;;
        --peer)     PEERS+=("$2"); shift 2 ;;
        --chain-id) CHAIN_ID="$2"; shift 2 ;;
        --name)     NAME="$2"; shift 2 ;;
        --help|-h)  usage ;;
        *)          log_error "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ $EUID -ne 0 ]]; then
    log_error "This script must be run as root (sudo)"
    exit 1
fi

if [[ -z "$KEY_FILE" ]]; then
    log_error "--key-file is required"
    exit 1
fi

if [[ ! -f "$KEY_FILE" ]]; then
    log_error "Key file not found: $KEY_FILE"
    exit 1
fi

log_info "Provisioning validator node: $NAME"
log_info "Chain ID: $CHAIN_ID"

# Create system user
if ! id viri &>/dev/null; then
    log_info "Creating system user 'viri'..."
    useradd --system --home-dir /var/lib/viri --shell /usr/sbin/nologin viri
fi

# Install dependencies
log_info "Installing dependencies..."
if command -v apt-get &>/dev/null; then
    apt-get update -qq
    apt-get install -y -qq ca-certificates curl jq
elif command -v yum &>/dev/null; then
    yum install -y ca-certificates curl jq
fi

# Install binary
if command -v virid &>/dev/null; then
    log_info "virid already installed at $(which virid)"
else
    log_info "Installing virid binary..."
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  GOARCH="amd64" ;;
        aarch64) GOARCH="arm64" ;;
        *)       log_error "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    LATEST_URL=$(curl -sf "https://api.github.com/repos/viri-chain/viri/releases/latest" | jq -r '.assets[] | select(.name | contains("linux-'$GOARCH'")) | .browser_download_url' | head -1)
    if [[ -n "$LATEST_URL" ]]; then
        curl -sL "$LATEST_URL" -o /tmp/viri.tar.gz
        tar -xzf /tmp/viri.tar.gz -C /tmp virid virictl
        mv /tmp/virid /usr/local/bin/virid
        mv /tmp/virictl /usr/local/bin/virictl
        chmod +x /usr/local/bin/virid /usr/local/bin/virictl
        rm -f /tmp/viri.tar.gz
        log_success "Installed from GitHub release"
    else
        log_warn "No pre-built binary found. Building from source..."
        if ! command -v go &>/dev/null; then
            log_info "Installing Go..."
            curl -sL "https://golang.org/dl/go1.21.6.linux-${GOARCH}.tar.gz" -o /tmp/go.tar.gz
            tar -C /usr/local -xzf /tmp/go.tar.gz
            rm /tmp/go.tar.gz
            export PATH=$PATH:/usr/local/go/bin
        fi
        log_info "Cloning repository..."
        git clone https://github.com/viri-chain/viri.git /tmp/viri-build
        cd /tmp/viri-build
        CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/virid ./cmd/virid
        CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/virictl ./cmd/virictl
        chmod +x /usr/local/bin/virid /usr/local/bin/virictl
        rm -rf /tmp/viri-build
        log_success "Built from source"
    fi
fi

# Setup directories
log_info "Setting up directories..."
mkdir -p /etc/viri /var/lib/viri /var/log/viri
chown -R viri:viri /var/lib/viri /var/log/viri

# Copy validator key
cp "$KEY_FILE" /etc/viri/validator.key
chmod 600 /etc/viri/validator.key
chown viri:viri /etc/viri/validator.key

# Generate config if not provided
if [[ -z "$CONFIG_FILE" ]]; then
    log_info "Generating node configuration..."

    PEERS_JSON="["
    FIRST=true
    for peer in "${PEERS[@]}"; do
        [[ "$FIRST" != "true" ]] && PEERS_JSON+=","
        PEERS_JSON+="\"$peer\""
        FIRST=false
    done
    PEERS_JSON+="]"

    cat > /etc/viri/config.json << EOF
{
  "chain": {
    "chain_id": $CHAIN_ID,
    "network_name": "viri-testnet",
    "block_time": "1s",
    "genesis_file": "/var/lib/viri/genesis.json"
  },
  "network": {
    "listen_addr": "0.0.0.0:30303",
    "bootstrap_peers": $PEERS_JSON,
    "max_peers": 100,
    "enable_dht": true,
    "enable_nat": true
  },
  "node": {
    "name": "$NAME",
    "data_dir": "/var/lib/viri",
    "validator_mode": true,
    "validator_key": "/etc/viri/validator.key",
    "rpc_enabled": true,
    "rpc_port": 8545,
    "api_enabled": true,
    "api_port": 8546,
    "metrics_enabled": true,
    "metrics_port": 9090
  },
  "consensus": {
    "min_stake": 10000,
    "epoch_length": 100,
    "slashing_enabled": true
  },
  "storage": {
    "backend": "leveldb",
    "path": "/var/lib/viri/chaindata",
    "pruning_enabled": true,
    "pruning_keep_recent": 100000
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout"
  }
}
EOF
else
    cp "$CONFIG_FILE" /etc/viri/config.json
fi

chown viri:viri /etc/viri/config.json

# Install systemd service
log_info "Installing systemd service..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(dirname "$SCRIPT_DIR")"

cp "$PARENT_DIR/systemd/virid.service" /etc/systemd/system/virid.service

# Enable and start
log_info "Enabling and starting virid..."
systemctl daemon-reload
systemctl enable virid
systemctl start virid

sleep 3

# Check status
if systemctl is-active --quiet virid; then
    log_success "Validator node started successfully!"
    log_info "View logs: journalctl -u virid -f"
    log_info "Check status: systemctl status virid"
    log_info "RPC endpoint: http://localhost:8545"
else
    log_error "Failed to start virid"
    log_info "Check logs: journalctl -u virid -e"
    exit 1
fi
