#!/usr/bin/env bash
# Viri Blockchain - Multi-Validator Testnet Initialization Script
# Creates a local testnet with N validator nodes
#
# Usage: ./testnet-init.sh [OPTIONS]
#   --validators N       Number of validators to create (default: 4)
#   --chain-id ID        Chain ID for the testnet (default: 1337)
#   --base-port PORT     Base port for P2P (default: 30303)
#   --output-dir DIR     Output directory for node data (default: ./testnet)
#   --stake AMOUNT       Initial stake per validator (default: 1000000)
#   --help               Show this help message

set -euo pipefail

# Default values
VALIDATORS=4
CHAIN_ID=1337
BASE_PORT=30303
OUTPUT_DIR="./testnet"
STAKE=1000000
VIRICTL_BIN="virictl"
VIRID_BIN="virid"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
    echo "Viri Blockchain - Multi-Validator Testnet Initialization Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --validators N       Number of validators to create (default: 4)"
    echo "  --chain-id ID        Chain ID for the testnet (default: 1337)"
    echo "  --base-port PORT     Base port for P2P (default: 30303)"
    echo "  --output-dir DIR     Output directory for node data (default: ./testnet)"
    echo "  --stake AMOUNT       Initial stake per validator (default: 1000000)"
    echo "  --virictl BIN        Path to virictl binary (default: virictl)"
    echo "  --virid BIN          Path to virid binary (default: virid)"
    echo "  --help               Show this help message"
    echo ""
    echo "Example:"
    echo "  $0 --validators 4 --chain-id 1337 --base-port 30303 --output-dir ./my-testnet"
    exit 0
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --validators)
            VALIDATORS="$2"
            shift 2
            ;;
        --chain-id)
            CHAIN_ID="$2"
            shift 2
            ;;
        --base-port)
            BASE_PORT="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --stake)
            STAKE="$2"
            shift 2
            ;;
        --virictl)
            VIRICTL_BIN="$2"
            shift 2
            ;;
        --virid)
            VIRID_BIN="$2"
            shift 2
            ;;
        --help|-h)
            usage
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate inputs
if [[ $VALIDATORS -lt 1 ]]; then
    log_error "Number of validators must be at least 1"
    exit 1
fi

if [[ $VALIDATORS -lt 4 ]]; then
    log_warn "HotStuff BFT requires minimum 4 validators for optimal fault tolerance"
fi

# Check if binaries exist
if ! command -v "$VIRICTL_BIN" &> /dev/null; then
    if [[ -f "./virictl.exe" ]]; then
        VIRICTL_BIN="./virictl.exe"
    elif [[ -f "./cmd/virictl/virictl.exe" ]]; then
        VIRICTL_BIN="./cmd/virictl/virictl.exe"
    else
        log_error "virictl binary not found. Please build it first or specify --virictl"
        exit 1
    fi
fi

log_info "Starting testnet initialization..."
log_info "Validators: $VALIDATORS"
log_info "Chain ID: $CHAIN_ID"
log_info "Base Port: $BASE_PORT"
log_info "Output Directory: $OUTPUT_DIR"

# Clean and create output directory
if [[ -d "$OUTPUT_DIR" ]]; then
    log_warn "Output directory already exists. Removing..."
    rm -rf "$OUTPUT_DIR"
fi

mkdir -p "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/genesis"

# Generate validator keys and configs
log_info "Generating validator keys and configurations..."

VALIDATOR_ADDRESSES=()
VALIDATOR_PUBKEYS=()
BOOTSTRAP_PEERS=()

for i in $(seq 0 $((VALIDATORS - 1))); do
    NODE_DIR="$OUTPUT_DIR/node-$i"
    CONFIG_DIR="$NODE_DIR/config"
    DATA_DIR="$NODE_DIR/data"
    KEYSTORE_DIR="$NODE_DIR/keystore"

    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$KEYSTORE_DIR"

    # Generate validator key using virictl wallet create
    # Since we can't interactively use virictl wallet create, we'll use Go to generate keys
    log_info "Generating keys for validator $i..."

    # Create a temporary Go script to generate ECDSA P-256 keypair
    TMP_KEY_GEN=$(mktemp /tmp/viri-keygen-XXXXXX.go)
    cat > "$TMP_KEY_GEN" << 'GOEOF'
package main

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
)

type KeyOutput struct {
    PrivateKey string `json:"private_key"`
    PublicKey  string `json:"public_key"`
    Address    string `json:"address"`
}

func main() {
    privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to generate key: %v\n", err)
        os.Exit(1)
    }

    pubKey := privKey.PublicKey
    pubKeyBytes := elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)
    
    // Hash the public key to get address (SHA-256)
    hash := sha256.Sum256(pubKeyBytes)
    address := hex.EncodeToString(hash[:20]) // Use first 20 bytes

    privKeyBytes := privKey.D.Bytes()
    
    output := KeyOutput{
        PrivateKey: hex.EncodeToString(privKeyBytes),
        PublicKey:  hex.EncodeToString(pubKeyBytes),
        Address:    address,
    }

    json.NewEncoder(os.Stdout).Encode(output)
}
GOEOF

    # Run key generation
    KEY_OUTPUT=$(go run "$TMP_KEY_GEN" 2>/dev/null || echo "FAILED")
    rm -f "$TMP_KEY_GEN"

    if [[ "$KEY_OUTPUT" == "FAILED" ]]; then
        log_error "Failed to generate keys for validator $i"
        exit 1
    fi

    # Parse key output
    PRIV_KEY=$(echo "$KEY_OUTPUT" | grep -o '"private_key":"[^"]*"' | cut -d'"' -f4)
    PUB_KEY=$(echo "$KEY_OUTPUT" | grep -o '"public_key":"[^"]*"' | cut -d'"' -f4)
    ADDRESS=$(echo "$KEY_OUTPUT" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)

    # Save private key to file
    echo "$PRIV_KEY" > "$KEYSTORE_DIR/validator.key"

    # Save public key info
    cat > "$KEYSTORE_DIR/validator.json" << EOF
{
    "address": "0x$ADDRESS",
    "public_key": "$PUB_KEY",
    "stake": $STAKE
}
EOF

    VALIDATOR_ADDRESSES+=("0x$ADDRESS")
    VALIDATOR_PUBKEYS+=("$PUB_KEY")

    # Calculate ports for this node
    P2P_PORT=$((BASE_PORT + i * 10))
    RPC_PORT=$((8545 + i))
    API_PORT=$((8546 + i))

    BOOTSTRAP_PEERS+=("/ip4/127.0.0.1/tcp/$P2P_PORT/p2p/$ADDRESS")

    # Create node configuration
    cat > "$CONFIG_DIR/config.json" << EOF
{
    "chain": {
        "chain_id": $CHAIN_ID,
        "network_name": "viri-testnet",
        "block_time": "1s",
        "max_block_size": 10485760,
        "max_gas_per_block": 30000000,
        "genesis_file": "$NODE_DIR/data/genesis.json"
    },
    "network": {
        "listen_addr": "0.0.0.0:$P2P_PORT",
        "external_addr": "127.0.0.1:$P2P_PORT",
        "bootstrap_peers": [],
        "max_peers": 50,
        "enable_dht": true,
        "enable_nat": false
    },
    "node": {
        "name": "validator-$i",
        "data_dir": "$DATA_DIR",
        "validator_mode": true,
        "validator_key": "$KEYSTORE_DIR/validator.key",
        "rpc_enabled": true,
        "rpc_port": $RPC_PORT,
        "api_enabled": true,
        "api_port": $API_PORT
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
        "path": "$DATA_DIR/chaindata",
        "max_state_size": 10737418240,
        "pruning_enabled": true,
        "pruning_keep_recent": 100000,
        "archive_mode": false
    },
    "logging": {
        "level": "info",
        "format": "text",
        "output": "$NODE_DIR/node.log",
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

    log_success "Created node-$i configuration (P2P: $P2P_PORT, RPC: $RPC_PORT, API: $API_PORT)"
done

# Create unified genesis.json
log_info "Creating unified genesis configuration..."

# Build bootstrap peers array (excluding self for each node, but we'll add all for simplicity)
BOOTSTRAP_JSON="["
for peer in "${BOOTSTRAP_PEERS[@]}"; do
    BOOTSTRAP_JSON+="\"$peer\","
done
BOOTSTRAP_JSON="${BOOTSTRAP_JSON%,}]"

# Create genesis validators array
GENESIS_VALIDATORS="["
for i in $(seq 0 $((VALIDATORS - 1))); do
    GENESIS_VALIDATORS+="{
        \"address\": \"${VALIDATOR_ADDRESSES[$i]}\",
        \"public_key\": \"${VALIDATOR_PUBKEYS[$i]}\",
        \"stake\": $STAKE,
        \"name\": \"validator-$i\"
    },"
done
GENESIS_VALIDATORS="${GENESIS_VALIDATORS%,}]"

# Create the genesis file
cat > "$OUTPUT_DIR/genesis/genesis.json" << EOF
{
    "chain_id": $CHAIN_ID,
    "network_name": "viri-testnet",
    "genesis_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "hash": "",
    "validators": $GENESIS_VALIDATORS,
    "total_stake": $((STAKE * VALIDATORS)),
    "version": "0.1.0"
}
EOF

# Copy genesis to each node's data directory
for i in $(seq 0 $((VALIDATORS - 1))); do
    cp "$OUTPUT_DIR/genesis/genesis.json" "$OUTPUT_DIR/node-$i/data/genesis.json"
done

# Update each node's config with bootstrap peers (all other nodes)
for i in $(seq 0 $((VALIDATORS - 1))); do
    NODE_CONFIG="$OUTPUT_DIR/node-$i/config/config.json"
    
    # Build peers list (all nodes except self)
    PEERS_JSON="["
    for j in $(seq 0 $((VALIDATORS - 1))); do
        if [[ $i -ne $j ]]; then
            P2P_PORT=$((BASE_PORT + j * 10))
            PEERS_JSON+="\"${BOOTSTRAP_PEERS[$j]}\","
        fi
    done
    PEERS_JSON="${PEERS_JSON%,}]"
    
    # Update the config with bootstrap peers using jq if available, otherwise use sed
    if command -v jq &> /dev/null; then
        jq ".network.bootstrap_peers = $PEERS_JSON" "$NODE_CONFIG" > "$NODE_CONFIG.tmp" && mv "$NODE_CONFIG.tmp" "$NODE_CONFIG"
    else
        # Simple replacement - this is a basic approach
        log_warn "jq not found. Bootstrap peers may need to be set manually."
    fi
done

# Create testnet summary
log_info "Generating testnet summary..."

cat > "$OUTPUT_DIR/summary.txt" << EOF
========================================
  Viri Blockchain Testnet Summary
========================================

Chain ID: $CHAIN_ID
Network: viri-testnet
Validators: $VALIDATORS
Total Stake: $((STAKE * VALIDATORS))

----------------------------------------
  Node Connection Information
----------------------------------------
EOF

for i in $(seq 0 $((VALIDATORS - 1))); do
    P2P_PORT=$((BASE_PORT + i * 10))
    RPC_PORT=$((8545 + i))
    API_PORT=$((8546 + i))
    
    cat >> "$OUTPUT_DIR/summary.txt" << EOF

Node $i (validator-$i):
  Data Directory: $OUTPUT_DIR/node-$i
  P2P Port: $P2P_PORT
  RPC Port: $RPC_PORT
  API Port: $API_PORT
  Address: ${VALIDATOR_ADDRESSES[$i]}
  RPC URL: http://localhost:$RPC_PORT
  API URL: http://localhost:$API_PORT

EOF
done

cat >> "$OUTPUT_DIR/summary.txt" << EOF
----------------------------------------
  Quick Start Commands
----------------------------------------

Start all nodes:
  ./scripts/testnet-start.sh --dir $OUTPUT_DIR

Stop all nodes:
  ./scripts/testnet-stop.sh --dir $OUTPUT_DIR

Check status:
  ./scripts/testnet-start.sh --dir $OUTPUT_DIR status

========================================
EOF

log_success "Testnet initialization complete!"
log_info "Summary saved to: $OUTPUT_DIR/summary.txt"
echo ""
cat "$OUTPUT_DIR/summary.txt"
