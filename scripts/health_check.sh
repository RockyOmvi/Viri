#!/bin/bash
# Viri Node Health Check Script
# Usage: ./health_check.sh [--rpc <url>] [--api-key <key>] [--verbose]

set -e

RPC_URL="http://localhost:8545"
API_URL="http://localhost:8546"
API_KEY=""
VERBOSE=false
EXIT_CODE=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

for i in "$@"; do
    case $i in
        --rpc)
            RPC_URL="$2"
            API_URL="${RPC_URL%:*}:8546"
            shift 2
            ;;
        --api-key)
            API_KEY="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        *)
            ;;
    esac
done

AUTH_HEADER=""
if [ -n "$API_KEY" ]; then
    AUTH_HEADER="-H X-API-Key: $API_KEY"
fi

log() {
    if [ "$VERBOSE" = true ]; then
        echo -e "$1"
    fi
}

pass() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    EXIT_CODE=1
}

fail() {
    echo -e "${RED}✗${NC} $1"
    EXIT_CODE=2
}

echo "====================================="
echo "  Viri Node Health Check"
echo "====================================="
echo ""

# 1. Process Check
echo "--- Process Status ---"
if systemctl is-active --quiet virid 2>/dev/null; then
    pass "Node process is running"
else
    if pgrep -f "virid" > /dev/null 2>&1; then
        pass "Node process is running (manual)"
    else
        fail "Node process is NOT running"
    fi
fi

# 2. RPC Health Check
echo ""
echo "--- RPC Health ---"
RPC_RESPONSE=$(curl -s --max-time 5 "$RPC_URL/health" 2>/dev/null || echo "FAILED")
if [ "$RPC_RESPONSE" != "FAILED" ]; then
    STATUS=$(echo "$RPC_RESPONSE" | jq -r '.status' 2>/dev/null || echo "unknown")
    HEIGHT=$(echo "$RPC_RESPONSE" | jq -r '.height' 2>/dev/null || echo "0")
    PEERS=$(echo "$RPC_RESPONSE" | jq -r '.peers' 2>/dev/null || echo "0")

    if [ "$STATUS" = "ok" ]; then
        pass "RPC is healthy"
    else
        warn "RPC status: $STATUS"
    fi

    log "  Block height: $HEIGHT"
    log "  Peers: $PEERS"

    if [ "$PEERS" = "0" ]; then
        warn "No peers connected"
    fi
else
    fail "RPC is not responding"
fi

# 3. Readiness Check
echo ""
echo "--- Readiness ---"
READY_RESPONSE=$(curl -s --max-time 5 "$RPC_URL/ready" 2>/dev/null || echo "FAILED")
if [ "$READY_RESPONSE" != "FAILED" ]; then
    READY_STATUS=$(echo "$READY_RESPONSE" | jq -r '.status' 2>/dev/null || echo "unknown")
    if [ "$READY_STATUS" = "ready" ]; then
        pass "Node is ready"
    else
        warn "Node is NOT ready ($READY_STATUS)"
        log "  Response: $READY_RESPONSE"
    fi
else
    fail "Readiness endpoint not responding"
fi

# 4. API Health Check
echo ""
echo "--- REST API ---"
API_RESPONSE=$(curl -s --max-time 5 "$API_URL/api/v1/status" 2>/dev/null || echo "FAILED")
if [ "$API_RESPONSE" != "FAILED" ]; then
    API_STATUS=$(echo "$API_RESPONSE" | jq -r '.height' 2>/dev/null || echo "unknown")
    if [ "$API_STATUS" != "unknown" ]; then
        pass "REST API is healthy"
    else
        warn "REST API returned unexpected response"
    fi
else
    warn "REST API is not responding (may be disabled)"
fi

# 5. Metrics Check
echo ""
echo "--- Metrics ---"
METRICS_RESPONSE=$(curl -s --max-time 5 "$RPC_URL/metrics" 2>/dev/null || echo "FAILED")
if [ "$METRICS_RESPONSE" != "FAILED" ]; then
    pass "Prometheus metrics endpoint accessible"
else
    warn "Metrics endpoint not accessible (may be localhost-only)"
fi

# 6. Disk Space
echo ""
echo "--- Disk Space ---"
DISK_USAGE=$(df -h /var/lib/viri 2>/dev/null | tail -1 | awk '{print $5}' || echo "N/A")
if [ "$DISK_USAGE" != "N/A" ]; then
    USAGE_PCT=$(echo "$DISK_USAGE" | tr -d '%')
    if [ "$USAGE_PCT" -lt 80 ]; then
        pass "Disk usage: $DISK_USAGE"
    elif [ "$USAGE_PCT" -lt 90 ]; then
        warn "Disk usage: $DISK_USAGE (high)"
    else
        fail "Disk usage: $DISK_USAGE (critical)"
    fi
else
    log "Disk check skipped (non-standard path)"
fi

# 7. Sync Status
echo ""
echo "--- Sync Status ---"
SYNC_RESPONSE=$(curl -s --max-time 5 \
    -X POST "$RPC_URL" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' \
    2>/dev/null || echo "FAILED")

if [ "$SYNC_RESPONSE" != "FAILED" ]; then
    SYNC_RESULT=$(echo "$SYNC_RESPONSE" | jq -r '.result' 2>/dev/null || echo "unknown")
    if [ "$SYNC_RESULT" = "false" ]; then
        pass "Node is fully synced"
    else
        CURRENT=$(echo "$SYNC_RESULT" | jq -r '.current_block' 2>/dev/null || echo "0")
        HIGHEST=$(echo "$SYNC_RESULT" | jq -r '.highest_block' 2>/dev/null || echo "0")
        PHASE=$(echo "$SYNC_RESULT" | jq -r '.phase' 2>/dev/null || echo "syncing")
        warn "Sync in progress: $PHASE ($CURRENT / $HIGHEST)"
    fi
else
    log "Sync status unavailable"
fi

# 8. Backup Check
echo ""
echo "--- Backup Status ---"
BACKUP_DIR="/var/lib/viri/backups"
if [ -d "$BACKUP_DIR" ]; then
    LATEST_BACKUP=$(ls -t "$BACKUP_DIR"/*.tar.gz 2>/dev/null | head -1)
    if [ -n "$LATEST_BACKUP" ]; then
        BACKUP_AGE=$(( $(date +%s) - $(stat -c %Y "$LATEST_BACKUP") ))
        BACKUP_HOURS=$(( BACKUP_AGE / 3600 ))
        BACKUP_SIZE=$(du -h "$LATEST_BACKUP" | cut -f1)

        if [ "$BACKUP_HOURS" -lt 24 ]; then
            pass "Latest backup: $BACKUP_SIZE ($BACKUP_HOURS hours ago)"
        elif [ "$BACKUP_HOURS" -lt 48 ]; then
            warn "Latest backup is $BACKUP_HOURS hours old"
        else
            fail "Latest backup is $BACKUP_HOURS hours old (stale)"
        fi
    else
        warn "No backups found"
    fi
else
    log "Backup directory not found"
fi

# 9. TLS Check (if configured)
echo ""
echo "--- TLS Configuration ---"
TLS_CERT="/etc/viri/tls/server.crt"
if [ -f "$TLS_CERT" ]; then
    EXPIRY=$(openssl x509 -enddate -noout -in "$TLS_CERT" 2>/dev/null | cut -d= -f2 || echo "unknown")
    if [ "$EXPIRY" != "unknown" ]; then
        EXPIRY_TS=$(date -d "$EXPIRY" +%s 2>/dev/null || echo "0")
        NOW_TS=$(date +%s)
        DAYS_LEFT=$(( (EXPIRY_TS - NOW_TS) / 86400 ))

        if [ "$DAYS_LEFT" -gt 30 ]; then
            pass "TLS certificate valid ($DAYS_LEFT days remaining)"
        elif [ "$DAYS_LEFT" -gt 0 ]; then
            warn "TLS certificate expires in $DAYS_LEFT days"
        else
            fail "TLS certificate has EXPIRED"
        fi
    fi
else
    warn "No TLS certificate found (running in HTTP mode)"
fi

# 10. Log Errors
echo ""
echo "--- Recent Errors ---"
ERROR_COUNT=$(journalctl -u virid --since "1 hour ago" --no-pager 2>/dev/null | grep -c -i "error\|fatal\|panic" || echo "0")
if [ "$ERROR_COUNT" = "0" ]; then
    pass "No errors in last hour"
elif [ "$ERROR_COUNT" -lt 10 ]; then
    warn "$ERROR_COUNT errors in last hour"
else
    fail "$ERROR_COUNT errors in last hour"
fi

echo ""
echo "====================================="
if [ $EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}  Overall: HEALTHY${NC}"
elif [ $EXIT_CODE -eq 1 ]; then
    echo -e "${YELLOW}  Overall: DEGRADED${NC}"
else
    echo -e "${RED}  Overall: UNHEALTHY${NC}"
fi
echo "====================================="

exit $EXIT_CODE
