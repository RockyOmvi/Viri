#!/bin/bash
# start-network.sh - Start Viri blockchain network with bootstrapping

set -e

echo "=== Viri Blockchain Network Startup ==="
echo ""

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

BOOTSTRAP_PEER_ID=""
MAX_RETRIES=30
RETRY_INTERVAL=2

get_bootstrap_peer_id() {
    echo "Waiting for bootstrap node to start..."

    for i in $(seq 1 $MAX_RETRIES); do
        echo -n "."

        PEER_ID=$(docker compose -f docker/docker-compose.yml logs bootstrap 2>/dev/null | grep -o "peer_id=[^ ]*" | head -1 | cut -d= -f2)

        if [ -n "$PEER_ID" ]; then
            echo ""
            BOOTSTRAP_PEER_ID="$PEER_ID"
            return 0
        fi

        sleep $RETRY_INTERVAL
    done

    echo ""
    echo "ERROR: Could not get bootstrap peer ID after $MAX_RETRIES retries"
    return 1
}

case "${1:-start}" in
    start)
        echo "Building containers..."
        docker compose -f docker/docker-compose.yml build

        echo "Starting bootstrap node..."
        docker compose -f docker/docker-compose.yml up -d bootstrap

        get_bootstrap_peer_id

        echo "Bootstrap peer ID: $BOOTSTRAP_PEER_ID"
        echo ""

        echo "Starting validators and explorer..."
        BOOTSTRAP_PEER_ID="$BOOTSTRAP_PEER_ID" docker compose -f docker/docker-compose.yml up -d

        echo ""
        echo "=== Network Started ==="
        echo "Bootstrap: localhost:30300"
        echo "Validator 1: localhost:30301 (RPC: localhost:8541)"
        echo "Validator 2: localhost:30302 (RPC: localhost:8542)"
        echo "Validator 3: localhost:30303 (RPC: localhost:8543)"
        echo "Validator 4: localhost:30304 (RPC: localhost:8544)"
        echo "Explorer: localhost:8545"
        echo ""
        echo "To check network status:"
        echo "  docker compose -f docker/docker-compose.yml ps"
        echo "  docker compose -f docker/docker-compose.yml logs -f"
        ;;

    stop)
        echo "Stopping network..."
        docker compose -f docker/docker-compose.yml down
        echo "Network stopped."
        ;;

    restart)
        $0 stop
        $0 start
        ;;

    clean)
        echo "Stopping and cleaning network..."
        docker compose -f docker/docker-compose.yml down -v
        docker compose -f docker/docker-compose.yml rm -f
        echo "Network cleaned."
        ;;

    status)
        echo "=== Network Status ==="
        docker compose -f docker/docker-compose.yml ps
        echo ""
        echo "=== Bootstrap Logs (last 20 lines) ==="
        docker compose -f docker/docker-compose.yml logs --tail=20 bootstrap
        ;;

    logs)
        docker compose -f docker/docker-compose.yml logs -f "${2:-}"
        ;;

    *)
        echo "Usage: $0 {start|stop|restart|clean|status|logs [service]}"
        exit 1
        ;;
esac
