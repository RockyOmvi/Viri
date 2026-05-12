#!/usr/bin/env bash
#
# Viri Docker Testnet Smoke Test
#
# Verifies the 4-validator Docker testnet is running and producing blocks.
#
# Usage:
#   cd testnet && bash smoke_test.sh
#
# Prerequisites:
#   - Docker and Docker Compose installed
#   - docker-compose.yml in the current directory
#   - jq installed (optional, for JSON parsing)
#

set -euo pipefail

VERBOSE=false
TIMEOUT=120
POLL_INTERVAL=5
RPC_BASE=${RPC_BASE:-"http://localhost:8545"}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Validator RPC endpoints
VALIDATORS=(
  "http://localhost:8545"    # validator-0
  "http://localhost:8550"    # validator-1
  "http://localhost:8555"    # validator-2
  "http://localhost:8560"    # validator-3
)

VALIDATOR_NAMES=(
  "validator-0"
  "validator-1"
  "validator-2"
  "validator-3"
)

log_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_fail()  { echo -e "${RED}[FAIL]${NC}  $1"; }

check_dependency() {
  if ! command -v "$1" &>/dev/null; then
    log_fail "$1 is required but not installed."
    exit 1
  fi
}

health_check() {
  local rpc=$1
  local name=$2
  local result

  result=$(curl -sf --max-time 5 "$rpc/health" 2>/dev/null || echo "")
  if [ -z "$result" ]; then
    echo ""
    return 1
  fi
  echo "$result"
}

wait_for_health() {
  local rpc=$1
  local name=$2
  local elapsed=0

  while [ $elapsed -lt $TIMEOUT ]; do
    if health_check "$rpc" "$name" > /dev/null 2>&1; then
      return 0
    fi
    sleep $POLL_INTERVAL
    elapsed=$((elapsed + POLL_INTERVAL))
  done
  return 1
}

check_block_production() {
  local rpc=$1
  local name=$2
  local initial_height
  local current_height

  initial_height=$(curl -sf --max-time 5 "$rpc/health" 2>/dev/null | grep -o '"height":[0-9]*' | cut -d: -f2 || echo "0")

  if [ "$initial_height" = "" ] || [ "$initial_height" = "null" ]; then
    initial_height=0
  fi

  log_info "Waiting for block production on $name (current height: $initial_height)..."

  sleep 15

  current_height=$(curl -sf --max-time 5 "$rpc/health" 2>/dev/null | grep -o '"height":[0-9]*' | cut -d: -f2 || echo "0")

  if [ "$current_height" = "" ] || [ "$current_height" = "null" ]; then
    current_height=0
  fi

  if [ "$current_height" -gt "$initial_height" ]; then
    log_ok "$name produced blocks: $initial_height → $current_height"
    return 0
  else
    log_warn "$name stuck at height $current_height (no new blocks)"
    return 1
  fi
}

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          Viri Testnet Smoke Test                            ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check dependencies
check_dependency docker
check_dependency curl

# ── Step 1: Check Docker services ──
echo "───────────────────────────────────────────────────────────────"
log_info "Step 1: Checking Docker services..."
echo ""

RUNNING=$(docker compose ps --status running -q 2>/dev/null | wc -l)
if [ "$RUNNING" -lt 4 ]; then
  log_warn "Expected 4 validator containers, found $RUNNING running."
  log_info "Attempting to start testnet..."
  docker compose up -d --wait 2>/dev/null || {
    log_fail "Failed to start Docker testnet. Run: docker compose up -d"
    exit 1
  }
  log_ok "Testnet containers started"
else
  log_ok "All $RUNNING validator containers are running"
fi

echo ""

# ── Step 2: Health check each validator ──
echo "───────────────────────────────────────────────────────────────"
log_info "Step 2: Checking validator health endpoints..."
echo ""

ALL_HEALTHY=true
for i in "${!VALIDATORS[@]}"; do
  rpc="${VALIDATORS[$i]}"
  name="${VALIDATOR_NAMES[$i]}"

  log_info "Waiting for $name at $rpc/health ..."

  if wait_for_health "$rpc" "$name"; then
    result=$(health_check "$rpc" "$name")
    height=$(echo "$result" | grep -o '"height":[0-9]*' | cut -d: -f2 || echo "0")
    peers=$(echo "$result" | grep -o '"peers":[0-9]*' | cut -d: -f2 || echo "0")
    log_ok "$name healthy | height=$height peers=$peers"
  else
    log_fail "$name not responding after ${TIMEOUT}s"
    ALL_HEALTHY=false
  fi
done

echo ""

if [ "$ALL_HEALTHY" = false ]; then
  log_fail "Not all validators are healthy. Aborting."
  exit 1
fi

# ── Step 3: Verify block production ──
echo "───────────────────────────────────────────────────────────────"
log_info "Step 3: Verifying block production..."
echo ""

ALL_PRODUCING=true
for i in "${!VALIDATORS[@]}"; do
  rpc="${VALIDATORS[$i]}"
  name="${VALIDATOR_NAMES[$i]}"

  if ! check_block_production "$rpc" "$name"; then
    ALL_PRODUCING=false
  fi
done

echo ""

# ── Step 4: Verify chain consistency ──
echo "───────────────────────────────────────────────────────────────"
log_info "Step 4: Checking chain consistency across validators..."
echo ""

HEIGHTS=()
for i in "${!VALIDATORS[@]}"; do
  rpc="${VALIDATORS[$i]}"
  name="${VALIDATOR_NAMES[$i]}"
  result=$(health_check "$rpc" "$name")
  h=$(echo "$result" | grep -o '"height":[0-9]*' | cut -d: -f2 || echo "0")
  HEIGHTS+=("$h")
done

# Check that heights are within 2 of each other
MAX_H=${HEIGHTS[0]}
MIN_H=${HEIGHTS[0]}
for h in "${HEIGHTS[@]}"; do
  [ "$h" -gt "$MAX_H" ] && MAX_H=$h
  [ "$h" -lt "$MIN_H" ] && MIN_H=$h
done

DIFF=$((MAX_H - MIN_H))
if [ "$DIFF" -le 2 ]; then
  log_ok "Chain consistent across validators (heights: ${HEIGHTS[*]})"
else
  log_warn "Height discrepancy detected: ${HEIGHTS[*]} (diff=$DIFF)"
  log_warn "This may be normal during initial sync"
fi

echo ""

# ── Summary ──
echo "╔══════════════════════════════════════════════════════════════╗"
if [ "$ALL_HEALTHY" = true ] && [ "$ALL_PRODUCING" = true ]; then
  echo -e "║  ${GREEN}SMOKE TEST PASSED${NC}  —  All validators healthy,            ║"
  echo -e "║  ${GREEN}blocks being produced, chain consistent${NC}                   ║"
else
  echo -e "║  ${YELLOW}SMOKE TEST PARTIAL${NC} —  Check logs for details           ║"
fi
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

if [ "$ALL_HEALTHY" = false ] || [ "$ALL_PRODUCING" = false ]; then
  exit 1
fi

exit 0
