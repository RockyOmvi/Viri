#!/usr/bin/env bash
# Viri Blockchain - Testnet Node Management Script
# Starts, stops, and manages testnet nodes created by testnet-init.sh
#
# Usage: ./testnet-start.sh [OPTIONS] [COMMAND]
# Commands: start, stop, restart, status
#   --dir DIR            Testnet directory (default: ./testnet)
#   --virid BIN          Path to virid binary (default: virid)
#   --node IDX           Start only specific node (optional)
#   --background         Run nodes in background (default)
#   --systemd            Use systemd services (Linux only)
#   --help               Show this help message

set -euo pipefail

# Default values
TESTNET_DIR="./testnet"
VIRID_BIN="virid"
COMMAND="start"
NODE_IDX=-1
USE_SYSTEMD=false
PID_DIR="/tmp/viri-testnet"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
    echo "Viri Blockchain - Testnet Node Management Script"
    echo ""
    echo "Usage: $0 [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start       Start all testnet nodes (default)"
    echo "  stop        Stop all testnet nodes"
    echo "  restart     Restart all testnet nodes"
    echo "  status      Show status of all testnet nodes"
    echo ""
    echo "Options:"
    echo "  --dir DIR            Testnet directory (default: ./testnet)"
    echo "  --virid BIN          Path to virid binary (default: virid)"
    echo "  --node IDX           Operate on specific node only"
    echo "  --background         Run nodes in background (default)"
    echo "  --systemd            Use systemd services (Linux only)"
    echo "  --help               Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 start                    # Start all nodes"
    echo "  $0 stop                     # Stop all nodes"
    echo "  $0 status                   # Check node status"
    echo "  $0 --node 0 start           # Start only node 0"
    echo "  $0 --dir ./my-testnet start # Start from custom directory"
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
        start|stop|restart|status)
            COMMAND="$1"
            shift
            ;;
        --dir)
            TESTNET_DIR="$2"
            shift 2
            ;;
        --virid)
            VIRID_BIN="$2"
            shift 2
            ;;
        --node)
            NODE_IDX="$2"
            shift 2
            ;;
        --background)
            USE_SYSTEMD=false
            shift
            ;;
        --systemd)
            USE_SYSTEMD=true
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
    log_info "Run ./scripts/testnet-init.sh first to create a testnet"
    exit 1
fi

# Check if virid binary exists
if ! command -v "$VIRID_BIN" &> /dev/null; then
    if [[ -f "./virid.exe" ]]; then
        VIRID_BIN="./virid.exe"
    elif [[ -f "./cmd/virid/virid.exe" ]]; then
        VIRID_BIN="./cmd/virid/virid.exe"
    else
        log_error "virid binary not found. Please build it first or specify --virid"
        exit 1
    fi
fi

# Create PID directory
mkdir -p "$PID_DIR"

# Get list of nodes
get_nodes() {
    if [[ $NODE_IDX -ge 0 ]]; then
        echo "node-$NODE_IDX"
    else
        ls -d "$TESTNET_DIR"/node-* 2>/dev/null | xargs -n1 basename
    fi
}

# Start a single node
start_node() {
    local NODE_NAME=$1
    local NODE_DIR="$TESTNET_DIR/$NODE_NAME"
    local CONFIG_FILE="$NODE_DIR/config/config.json"
    local PID_FILE="$PID_DIR/$NODE_NAME.pid"
    local LOG_FILE="$NODE_DIR/node.log"

    if [[ ! -d "$NODE_DIR" ]]; then
        log_error "Node directory not found: $NODE_DIR"
        return 1
    fi

    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_error "Config file not found: $CONFIG_FILE"
        return 1
    fi

    # Check if already running
    if [[ -f "$PID_FILE" ]]; then
        local OLD_PID=$(cat "$PID_FILE")
        if kill -0 "$OLD_PID" 2>/dev/null; then
            log_warn "$NODE_NAME is already running (PID: $OLD_PID)"
            return 0
        else
            rm -f "$PID_FILE"
        fi
    fi

    log_info "Starting $NODE_NAME..."

    # Start the node in background
    nohup "$VIRID_BIN" --config "$CONFIG_FILE" >> "$LOG_FILE" 2>&1 &
    local PID=$!
    echo "$PID" > "$PID_FILE"

    # Wait a moment and check if process is still running
    sleep 2
    if kill -0 "$PID" 2>/dev/null; then
        log_success "$NODE_NAME started (PID: $PID)"
        return 0
    else
        log_error "$NODE_NAME failed to start. Check $LOG_FILE for details."
        rm -f "$PID_FILE"
        return 1
    fi
}

# Stop a single node
stop_node() {
    local NODE_NAME=$1
    local PID_FILE="$PID_DIR/$NODE_NAME.pid"

    if [[ ! -f "$PID_FILE" ]]; then
        log_warn "$NODE_NAME is not running (no PID file)"
        return 0
    fi

    local PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        log_info "Stopping $NODE_NAME (PID: $PID)..."
        
        # Try graceful shutdown first
        kill -TERM "$PID" 2>/dev/null
        
        # Wait for process to terminate (up to 30 seconds)
        local COUNT=0
        while kill -0 "$PID" 2>/dev/null && [[ $COUNT -lt 30 ]]; do
            sleep 1
            COUNT=$((COUNT + 1))
        done
        
        # Force kill if still running
        if kill -0 "$PID" 2>/dev/null; then
            log_warn "Process did not terminate gracefully, forcing..."
            kill -9 "$PID" 2>/dev/null
            sleep 1
        fi
        
        log_success "$NODE_NAME stopped"
    else
        log_warn "$NODE_NAME was not running (stale PID file)"
    fi
    
    rm -f "$PID_FILE"
    return 0
}

# Get status of a single node
status_node() {
    local NODE_NAME=$1
    local NODE_DIR="$TESTNET_DIR/$NODE_NAME"
    local CONFIG_FILE="$NODE_DIR/config/config.json"
    local PID_FILE="$PID_DIR/$NODE_NAME.pid"

    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_error "$NODE_NAME: Config not found"
        return 1
    fi

    # Get ports from config
    local RPC_PORT=$(grep -o '"rpc_port":[[:space:]]*[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*')
    local P2P_PORT=$(grep -o '"listen_addr":[[:space:]]*"[^"]*' "$CONFIG_FILE" | grep -o '[0-9]*$')

    echo "----------------------------------------"
    echo "$NODE_NAME:"
    
    if [[ -f "$PID_FILE" ]]; then
        local PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            echo -e "  Status:   ${GREEN}RUNNING${NC} (PID: $PID)"
            
            # Try to get RPC status
            if [[ -n "$RPC_PORT" ]]; then
                local RPC_RESPONSE=$(curl -s --max-time 3 "http://localhost:$RPC_PORT/health" 2>/dev/null || echo "FAILED")
                if [[ "$RPC_RESPONSE" != "FAILED" ]]; then
                    echo "  RPC:       Healthy (http://localhost:$RPC_PORT)"
                else
                    echo "  RPC:       Not responding"
                fi
            fi
            
            echo "  P2P Port:  $P2P_PORT"
            echo "  Log:       $NODE_DIR/node.log"
        else
            echo -e "  Status:   ${RED}NOT RUNNING${NC} (stale PID file)"
            rm -f "$PID_FILE"
        fi
    else
        echo -e "  Status:   ${YELLOW}STOPPED${NC}"
    fi
    
    return 0
}

# Execute command
case $COMMAND in
    start)
        log_info "Starting testnet nodes from: $TESTNET_DIR"
        FAILED=0
        for NODE in $(get_nodes); do
            if ! start_node "$NODE"; then
                FAILED=1
            fi
        done
        
        if [[ $FAILED -eq 0 ]]; then
            log_success "All nodes started successfully!"
            log_info "Check status with: $0 status --dir $TESTNET_DIR"
        else
            log_error "Some nodes failed to start"
            exit 1
        fi
        ;;
    stop)
        log_info "Stopping testnet nodes..."
        for NODE in $(get_nodes); do
            stop_node "$NODE"
        done
        log_success "All nodes stopped"
        ;;
    restart)
        log_info "Restarting testnet nodes..."
        for NODE in $(get_nodes); do
            stop_node "$NODE"
        done
        sleep 2
        for NODE in $(get_nodes); do
            start_node "$NODE"
        done
        log_success "All nodes restarted"
        ;;
    status)
        echo "========================================"
        echo "  Viri Testnet Status"
        echo "========================================"
        echo ""
        for NODE in $(get_nodes); do
            status_node "$NODE"
        done
        echo ""
        echo "========================================"
        ;;
    *)
        log_error "Unknown command: $COMMAND"
        usage
        ;;
esac
