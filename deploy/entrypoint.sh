#!/bin/sh
# Viri Blockchain - Docker Entrypoint Script
# Handles genesis initialization and node startup

set -e

VIRI_HOME="/home/viri"
CONFIG_DIR="${VIRI_HOME}/config"
DATA_DIR="${VIRI_DATA_DIR:-${VIRI_HOME}/.viri}"
GENESIS_FILE="${DATA_DIR}/genesis.json"
CONFIG_FILE="${CONFIG_DIR}/config.json"

# Colors (for docker logs)
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

log() {
    echo "${BLUE}[entrypoint]${NC} $1"
}

log_success() {
    echo "${GREEN}[entrypoint]${NC} $1"
}

log_warn() {
    echo "${YELLOW}[entrypoint]${NC} $1"
}

# Ensure directories exist
mkdir -p "${DATA_DIR}" "${CONFIG_DIR}"

# Copy genesis from data mount if not in expected location
ALT_GENESIS="/home/viri/data/genesis.json"
if [ -f "${ALT_GENESIS}" ] && [ ! -f "${GENESIS_FILE}" ]; then
    cp "${ALT_GENESIS}" "${GENESIS_FILE}"
    log_success "Genesis copied from mount"
fi

# Apply environment variable overrides to config
if [ -f "${CONFIG_FILE}" ]; then
    # Copy to writable location (source may be read-only mount)
    WORK_CONFIG="${DATA_DIR}/config.json"
    if [ ! -f "${WORK_CONFIG}" ] || [ "${CONFIG_FILE}" -nt "${WORK_CONFIG}" ]; then
        cp "${CONFIG_FILE}" "${WORK_CONFIG}"
    fi

    if command -v jq > /dev/null 2>&1; then
        log "Applying environment overrides..."

        TMP_CONFIG=$(mktemp)

        jq '
            (if env.VIRI_NODE_NAME != "" and env.VIRI_NODE_NAME != null then .node.name = env.VIRI_NODE_NAME else . end) |
            (if env.VIRI_DATA_DIR != "" and env.VIRI_DATA_DIR != null then .node.data_dir = env.VIRI_DATA_DIR else . end) |
            (if env.VIRI_RPC_PORT != "" and env.VIRI_RPC_PORT != null then .node.rpc_port = (env.VIRI_RPC_PORT | tonumber) else . end) |
            (if env.VIRI_API_PORT != "" and env.VIRI_API_PORT != null then .node.api_port = (env.VIRI_API_PORT | tonumber) else . end) |
            (if env.VIRI_VALIDATOR_MODE == "true" then .node.validator_mode = true elif env.VIRI_VALIDATOR_MODE == "false" then .node.validator_mode = false else . end) |
            (if env.VIRI_VALIDATOR_KEY != "" and env.VIRI_VALIDATOR_KEY != null then .node.validator_key = env.VIRI_VALIDATOR_KEY else . end) |
            (if env.VIRI_BOOTSTRAP_PEER != "" and env.VIRI_BOOTSTRAP_PEER != null then .p2p.bootstrap_peers = [env.VIRI_BOOTSTRAP_PEER] else . end) |
            (if env.VIRI_CHAIN_ID != "" and env.VIRI_CHAIN_ID != null then .chain.chain_id = (env.VIRI_CHAIN_ID | tonumber) else . end) |
            (if env.VIRI_LOG_LEVEL != "" and env.VIRI_LOG_LEVEL != null then .logging.level = env.VIRI_LOG_LEVEL else . end)
        ' "${WORK_CONFIG}" > "${TMP_CONFIG}"

        mv "${TMP_CONFIG}" "${WORK_CONFIG}"
        CONFIG_FILE="${WORK_CONFIG}"
    fi
    log_success "Configuration updated"
fi

# Build L3 port flag
L3_PORT_FLAG=""
if [ -n "${VIRI_L3_PORT}" ]; then
    L3_PORT_FLAG="--l3-port ${VIRI_L3_PORT}"
fi

# Detect special modes from any argument position
# Docker compose command: ["/usr/local/bin/virid", "--explorer"]
# entrypoint receives these as $1, $2, etc.
MODE=""
for arg in "$@"; do
    case "$arg" in
        --explorer)
            MODE="explorer"
            ;;
        --faucet)
            MODE="faucet"
            ;;
        --genesis-init)
            MODE="genesis-init"
            ;;
        --testnet)
            TESTNET_FLAG="--testnet"
            ;;
        --help)
            MODE="help"
            ;;
    esac
done

case "${MODE}" in
    explorer)
        log "Starting block explorer..."
        exec virid --explorer
        ;;
    faucet)
        log "Starting faucet service..."
        exec virid --faucet
        ;;
    genesis-init)
        log "Running genesis initialization..."
        if [ ! -f "${GENESIS_FILE}" ]; then
            log "Creating default genesis..."
            cat > "${GENESIS_FILE}" << EOF
{
    "chain_id": ${VIRI_CHAIN_ID:-1337},
    "network_name": "viri-testnet",
    "genesis_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "hash": "",
    "validators": [],
    "total_stake": 0,
    "version": "0.1.0"
}
EOF
            log_success "Genesis created at ${GENESIS_FILE}"
        else
            log "Genesis already exists, skipping"
        fi
        exit 0
        ;;
    help)
        echo "Viri Blockchain Node"
        echo ""
        echo "Usage: virid [OPTIONS]"
        echo ""
        echo "Special modes:"
        echo "  --genesis-init      Initialize genesis configuration"
        echo "  --explorer          Start block explorer mode"
        echo "  --faucet            Start faucet service"
        echo "  --testnet           Shortcut for testnet config (chain ID 2)"
        echo ""
        echo "Environment variables:"
        echo "  VIRI_NODE_NAME        Node name"
        echo "  VIRI_DATA_DIR         Data directory"
        echo "  VIRI_RPC_PORT         RPC port"
        echo "  VIRI_API_PORT         API port"
        echo "  VIRI_LOG_LEVEL        Log level (debug, info, warn, error)"
        echo "  VIRI_VALIDATOR_MODE   Enable validator mode"
        echo "  VIRI_VALIDATOR_KEY    Path to validator key file"
        echo "  VIRI_CHAIN_ID         Chain ID"
        echo "  VIRI_TESTNET          Set to true for testnet mode"
        echo "  VIRI_CONSENSUS_DELAY  Delay before starting consensus"
        echo "  VIRI_KEY_PASSPHRASE   Passphrase for key encryption"
        echo ""
        echo "Auto-generated API key:"
        echo "  When no api_key_hash is set, the node generates a random 32-byte"
        echo "  API key on first run, persists it to {dataDir}/api_key.txt,"
        exit 0
        ;;
esac

# ---- Normal validator startup ----

# Check if genesis exists
if [ ! -f "${GENESIS_FILE}" ]; then
    log_warn "No genesis file found at ${GENESIS_FILE}"
    log "Run with --genesis-init first or mount genesis.json"
    exit 1
fi

log "Starting Viri node..."
log "  Config: ${CONFIG_FILE}"
log "  Data:   ${DATA_DIR}"
log "  Genesis: ${GENESIS_FILE}"

# Check validator key
if [ -n "${VIRI_VALIDATOR_KEY}" ]; then
    if [ -f "${VIRI_VALIDATOR_KEY}" ]; then
        log_success "Validator key found at ${VIRI_VALIDATOR_KEY}"
    else
        log_warn "Validator key path set but file not found: ${VIRI_VALIDATOR_KEY}"
    fi
fi

# Build validator flag from config
VALIDATOR_FLAG=""
CONSENSUS_DELAY=""
if command -v jq > /dev/null 2>&1 && [ -f "${CONFIG_FILE}" ]; then
    if [ "$(jq -r '.node.validator_mode // false' "${CONFIG_FILE}")" = "true" ]; then
        VALIDATOR_FLAG="--validator"
        # Add consensus delay to allow P2P mesh to form before starting consensus
        DELAY_FROM_CONFIG=$(jq -r '.consensus.startup_delay // ""' "${CONFIG_FILE}" 2>/dev/null)
        if [ -n "${DELAY_FROM_CONFIG}" ] && [ "${DELAY_FROM_CONFIG}" != "null" ]; then
            CONSENSUS_DELAY="--consensus-delay ${DELAY_FROM_CONFIG}"
        elif [ -n "${VIRI_CONSENSUS_DELAY}" ]; then
            CONSENSUS_DELAY="--consensus-delay ${VIRI_CONSENSUS_DELAY}"
        else
            CONSENSUS_DELAY="--consensus-delay 30s"
        fi
    fi
fi

# Parse extra args — strip out virid and --config from CMD since entrypoint provides them
EXTRA_ARGS=""
SKIP_NEXT=false
for arg in "$@"; do
    if $SKIP_NEXT; then
        SKIP_NEXT=false
        continue
    fi
    case "$arg" in
        virid|/usr/local/bin/virid)
            continue
            ;;
        --config)
            SKIP_NEXT=true
            continue
            ;;
        --testnet)
            continue
            ;;
    esac
    EXTRA_ARGS="${EXTRA_ARGS} ${arg}"
done

# Check if testnet mode is requested (env var or flag)
TESTNET_FLAG=""
if [ "${VIRI_TESTNET}" = "true" ]; then
    TESTNET_FLAG="--testnet"
fi

log "Flags: ${VALIDATOR_FLAG} ${CONSENSUS_DELAY} ${L3_PORT_FLAG} ${TESTNET_FLAG} ${EXTRA_ARGS}"

# Execute the main command in background, then run peer discovery
virid --config "${CONFIG_FILE}" ${VALIDATOR_FLAG} ${CONSENSUS_DELAY} ${L3_PORT_FLAG} ${TESTNET_FLAG} ${EXTRA_ARGS} &
VIRID_PID=$!

# Run peer discovery in background if we have multiple validators
PEER_DISCOVERY="${VIRI_HOME}/peer-discovery.sh"
if [ -f "${PEER_DISCOVERY}" ] && [ -n "${VIRI_NODE_INDEX+x}" ]; then
    log "Starting peer discovery in background (index=${VIRI_NODE_INDEX})..."
    sh "${PEER_DISCOVERY}" "${VIRI_NODE_INDEX}" &
fi

# Wait for virid to exit
wait $VIRID_PID
