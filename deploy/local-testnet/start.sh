#!/usr/bin/env bash
# Viri Local Testnet - One-command launcher
# Usage: bash deploy/local-testnet/start.sh
set -euo pipefail

cd "$(dirname "$0")/../.."

echo "==> Building Docker image..."
docker build -t viri-chain:latest -f Dockerfile .

echo ""
echo "==> Starting local testnet (4 validators + explorer + faucet)..."
docker compose -f docker-compose.yml up -d

echo ""
echo "==> Waiting for validators to become healthy..."
for i in 0 1 2 3; do
    echo -n "  validator-${i} ... "
    until docker inspect --format='{{.State.Health.Status}}' viri-validator-${i} 2>/dev/null | grep -q healthy; do
        sleep 2
    done
    echo "healthy"
done

echo ""
echo "==> Local testnet is running!"
echo "  RPC:        http://localhost:8545"
echo "  API:        http://localhost:8546"
echo "  Explorer:   http://localhost:8080"
echo "  Faucet:     http://localhost:8081"
echo "  Grafana:    http://localhost:3000 (admin/admin)"
echo "  Prometheus: http://localhost:9091"
echo ""
echo "  Validator ports: 30303 (v0), 30304 (v1), 30305 (v2), 30306 (v3)"
echo "  RPC ports:       8545 (v0), 8547 (v1), 8548 (v2), 8549 (v3)"
echo ""
echo "==> To stop: docker compose -f docker-compose.yml down"
echo "==> To view logs: docker compose -f docker-compose.yml logs -f"
