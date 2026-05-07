#!/usr/bin/env bash
# Viri Blockchain - Testnet Stop Script
# Gracefully stops all testnet nodes started by testnet-start.sh
#
# Usage: ./testnet-stop.sh [OPTIONS]
#   --dir DIR            Testnet directory (default: ./testnet)
#   --force              Force kill nodes if graceful stop fails
#   --clean              Remove PID files and log files after stopping
#   --help               Show this help message

set -euo pipefail

# Default values
TESTNET_DIR="./testnet"
FORCE_STOP=false
CLEAN=false
PID_DIR="/tmp/viri-testnet"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
    echo "Viri Blockchain - Testnet Stop Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --dir DIR            Testnet directory (default: ./testnet)"
    echo "  --force              Force kill nodes if graceful stop fails"
    echo "  --clean              Remove PID files and log files after stopping"
    echo "  --help               Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                           # Stop all nodes"
    echo "  $0 --dir ./my-testnet        # Stop from custom directory"
    echo "  $0 --force --clean           # Force stop and clean up"
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
        --dir)
            TESTNET_DIR="$2"
            shift 2
            ;;
        --force)
            FORCE_STOP=true
            shift
            ;;
        --clean)
            CLEAN=true
            shift
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

# Check if testnet directory exists
if [[ ! -d "$TESTNET_DIR" ]]; then
    log_error "Testnet directory not found: $TESTNET_DIR"
    exit 1
fi

# Check if PID directory exists
if [[ ! -d "$PID_DIR" ]]; then
    log_warn "PID directory not found: $PID_DIR"
    log_info "No nodes appear to be running"
    exit 0
fi

log_info "Stopping testnet nodes from: $TESTNET_DIR"

# Get list of running nodes from PID files
RUNNING_NODES=()
for PID_FILE in "$PID_DIR"/node-*.pid; do
    if [[ -f "$PID_FILE" ]]; then
        NODE_NAME=$(basename "$PID_FILE" .pid)
        PID=$(cat "$PID_FILE")
        
        if kill -0 "$PID" 2>/dev/null; then
            RUNNING_NODES+=("$NODE_NAME:$PID")
        else
            log_warn "$NODE_NAME has stale PID file (PID: $PID not running)"
            rm -f "$PID_FILE"
        fi
    fi
done

if [[ ${#RUNNING_NODES[@]} -eq 0 ]]; then
    log_info "No running nodes found"
    exit 0
fi

log_info "Found ${#RUNNING_NODES[@]} running node(s)"

# Stop each node
STOPPED=0
FAILED=0

for NODE_INFO in "${RUNNING_NODES[@]}"; do
    NODE_NAME=$(echo "$NODE_INFO" | cut -d':' -f1)
    PID=$(echo "$NODE_INFO" | cut -d':' -f2)
    PID_FILE="$PID_DIR/$NODE_NAME.pid"
    
    log_info "Stopping $NODE_NAME (PID: $PID)..."
    
    # Try graceful shutdown with SIGTERM
    kill -TERM "$PID" 2>/dev/null
    
    # Wait for process to terminate (up to 30 seconds)
    COUNT=0
    while kill -0 "$PID" 2>/dev/null && [[ $COUNT -lt 30 ]]; do
        sleep 1
        COUNT=$((COUNT + 1))
        
        # Show progress every 5 seconds
        if [[ $((COUNT % 5)) -eq 0 ]]; then
            log_info "  Waiting for $NODE_NAME to stop... ($COUNT/30 seconds)"
        fi
    done
    
    # Check if process stopped
    if kill -0 "$PID" 2>/dev/null; then
        if [[ "$FORCE_STOP" == true ]]; then
            log_warn "$NODE_NAME did not stop gracefully, force killing..."
            kill -9 "$PID" 2>/dev/null
            sleep 1
            
            if kill -0 "$PID" 2>/dev/null; then
                log_error "$NODE_NAME could not be killed"
                FAILED=$((FAILED + 1))
            else
                log_success "$NODE_NAME force stopped"
                rm -f "$PID_FILE"
                STOPPED=$((STOPPED + 1))
            fi
        else
            log_error "$NODE_NAME did not stop gracefully (use --force to force kill)"
            FAILED=$((FAILED + 1))
        fi
    else
        log_success "$NODE_NAME stopped gracefully"
        rm -f "$PID_FILE"
        STOPPED=$((STOPPED + 1))
    fi
done

# Clean up if requested
if [[ "$CLEAN" == true ]]; then
    log_info "Cleaning up..."
    
    # Remove PID files
    rm -f "$PID_DIR"/node-*.pid
    
    # Remove log files
    for NODE_DIR in "$TESTNET_DIR"/node-*; do
        if [[ -d "$NODE_DIR" ]]; then
            NODE_NAME=$(basename "$NODE_DIR")
            LOG_FILE="$NODE_DIR/node.log"
            if [[ -f "$LOG_FILE" ]]; then
                rm -f "$LOG_FILE"
                log_info "Removed $NODE_NAME log file"
            fi
        fi
    done
    
    # Remove PID directory if empty
    if [[ -d "$PID_DIR" ]] && [[ -z "$(ls -A "$PID_DIR" 2>/dev/null)" ]]; then
        rmdir "$PID_DIR" 2>/dev/null || true
    fi
    
    log_success "Cleanup complete"
fi

# Summary
echo ""
echo "========================================"
echo "  Stop Summary"
echo "========================================"
echo "  Nodes stopped: $STOPPED"
echo "  Nodes failed:  $FAILED"
echo "========================================"

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi

log_success "All testnet nodes stopped successfully"
