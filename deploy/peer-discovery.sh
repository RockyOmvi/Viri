#!/bin/sh
# Peer Discovery Script - Connects validators to each other via RPC
# Usage: peer-discovery.sh [own_index]
# Example: peer-discovery.sh 0 (for validator-0)

set -e

OWN_INDEX="${1:-0}"
RPC_PORT=8545
MAX_RETRIES=60
RETRY_DELAY=2
API_KEY_FILE="/home/viri/.viri/api_key.txt"

# Validator service names
VALIDATORS="validator-0 validator-1 validator-2 validator-3"

API_KEY=""

log() {
    echo "[peer-discovery] $1"
}

load_api_key() {
    if [ -f "$API_KEY_FILE" ]; then
        API_KEY=$(cat "$API_KEY_FILE" 2>/dev/null)
    fi
}

rpc_call() {
    local host="$1"
    local method="$2"
    local params="$3"
    
    if [ -n "$API_KEY" ]; then
        curl -sf -X POST "http://${host}:${RPC_PORT}" \
            -H "Content-Type: application/json" \
            -H "X-API-Key: ${API_KEY}" \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params},\"id\":1}" 2>/dev/null
    else
        curl -sf -X POST "http://${host}:${RPC_PORT}" \
            -H "Content-Type: application/json" \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params},\"id\":1}" 2>/dev/null
    fi
}

wait_for_ready() {
    local host="$1"
    local retries=0
    
    while [ $retries -lt $MAX_RETRIES ]; do
        if rpc_call "$host" "eth_blockNumber" "[]" >/dev/null 2>&1; then
            return 0
        fi
        retries=$((retries + 1))
        sleep $RETRY_DELAY
    done
    
    return 1
}

log "Starting peer discovery (own index: ${OWN_INDEX})"

# Build list of peer services to connect to
PEER_SERVICES=""
IDX=0
for validator in $VALIDATORS; do
    if [ $IDX -ne $OWN_INDEX ]; then
        PEER_SERVICES="${PEER_SERVICES} ${validator}"
    fi
    IDX=$((IDX + 1))
done

log "Will connect to peers:${PEER_SERVICES}"

# Connect to each peer
for peer in $PEER_SERVICES; do
    log "Discovering peer: ${peer}..."
    
    # Wait for peer to be ready
    if ! wait_for_ready "$peer"; then
        log "WARN: Peer ${peer} not ready, skipping"
        continue
    fi
    
    # Load API key after RPC is ready (file created by virid during startup)
    load_api_key
    if [ -n "$API_KEY" ]; then
        log "API key loaded"
    fi
    
    # Get peer's full multiaddress via nodeInfo
    log "Querying ${peer} for multiaddress..."
    
    RESULT=$(rpc_call "$peer" "viri_nodeInfo" "[]" 2>/dev/null || true)
    
    if [ -z "$RESULT" ]; then
        log "WARN: Could not get node info from ${peer}"
        continue
    fi
    
    # Try to get multiaddr directly from the response
    MULTIADDR=$(echo "$RESULT" | grep -o '"multiaddr":"[^"]*"' | cut -d'"' -f4)
    
    # If no multiaddr field, construct from full_peer_id
    if [ -z "$MULTIADDR" ]; then
        FULL_PEER_ID=$(echo "$RESULT" | grep -o '"full_peer_id":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$FULL_PEER_ID" ]; then
            MULTIADDR="/dns/${peer}/tcp/30303/p2p/${FULL_PEER_ID}"
        fi
    fi
    
    if [ -z "$MULTIADDR" ]; then
        log "WARN: No peer info from ${peer}"
        continue
    fi
    
    log "Connecting to ${peer}: ${MULTIADDR}"
    
    # Connect via RPC
    CONNECT_RESULT=$(rpc_call "localhost" "viri_addPeer" "[\"${MULTIADDR}\"]" 2>/dev/null || true)
    
    if echo "$CONNECT_RESULT" | grep -q '"success":true'; then
        log "SUCCESS: Connected to ${peer}"
    else
        ERROR=$(echo "$CONNECT_RESULT" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$ERROR" ]; then
            log "WARN: Failed to connect to ${peer}: ${ERROR}"
        else
            log "WARN: Failed to connect to ${peer}"
        fi
    fi
    
    # Small delay between connections
    sleep 2
done

log "Peer discovery complete"
