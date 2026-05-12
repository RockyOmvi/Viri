#!/usr/bin/env bash
# Stop local testnet
cd "$(dirname "$0")/../.."
docker compose -f docker-compose.yml down
echo "Testnet stopped."
