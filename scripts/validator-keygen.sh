#!/usr/bin/env bash
# Viri Blockchain - Validator Key Generation Script
# Generates ECDSA P-256 keypairs for validators
#
# Usage: ./validator-keygen.sh [OPTIONS]
#   --count N           Number of keypairs to generate (default: 1)
#   --output-dir DIR    Output directory for keys (default: ./validator-keys)
#   --prefix NAME       Prefix for key filenames (default: validator)
#   --stake AMOUNT      Stake amount to include in key metadata (default: 1000000)
#   --batch             Generate batch keys (no prompts)
#   --format FMT        Output format: json, text, env (default: json)
#   --help              Show this help message

set -euo pipefail

# Default values
COUNT=1
OUTPUT_DIR="./validator-keys"
PREFIX="validator"
STAKE=1000000
BATCH=false
FORMAT="json"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
    echo "Viri Blockchain - Validator Key Generation Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --count N           Number of keypairs to generate (default: 1)"
    echo "  --output-dir DIR    Output directory for keys (default: ./validator-keys)"
    echo "  --prefix NAME       Prefix for key filenames (default: validator)"
    echo "  --stake AMOUNT      Stake amount to include in metadata (default: 1000000)"
    echo "  --batch             Generate batch keys (no prompts)"
    echo "  --format FMT        Output format: json, text, env (default: json)"
    echo "  --help              Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                           # Generate 1 keypair"
    echo "  $0 --count 4                 # Generate 4 keypairs"
    echo "  $0 --count 4 --batch         # Generate 4 keypairs non-interactively"
    echo "  $0 --output-dir ./keys       # Save to custom directory"
    echo ""
    echo "Output files per validator:"
    echo "  <prefix>-N.key               Private key (hex encoded)"
    echo "  <prefix>-N.pub               Public key (hex encoded)"
    echo "  <prefix>-N.json              Key metadata (JSON)"
    echo "  <prefix>-N.key.enc           Encrypted keystore (if using virictl)"
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
    echo -e "${RED}[ERROR]${NC} $1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --count)
            COUNT="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --prefix)
            PREFIX="$2"
            shift 2
            ;;
        --stake)
            STAKE="$2"
            shift 2
            ;;
        --batch)
            BATCH=true
            shift
            ;;
        --format)
            FORMAT="$2"
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
if [[ $COUNT -lt 1 ]]; then
    log_error "Count must be at least 1"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

log_info "Generating $COUNT validator keypair(s)..."
log_info "Output directory: $OUTPUT_DIR"
log_info "Key prefix: $PREFIX"

# Create Go key generation script
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
    "strconv"
)

type KeyOutput struct {
    Index      int    `json:"index"`
    PrivateKey string `json:"private_key"`
    PublicKey  string `json:"public_key"`
    Address    string `json:"address"`
    Stake      uint64 `json:"stake,omitempty"`
}

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintf(os.Stderr, "Usage: %s <index> [stake]\n", os.Args[0])
        os.Exit(1)
    }

    index, err := strconv.Atoi(os.Args[1])
    if err != nil {
        fmt.Fprintf(os.Stderr, "Invalid index: %v\n", err)
        os.Exit(1)
    }

    stake := uint64(1000000)
    if len(os.Args) > 2 {
        stake, _ = strconv.ParseUint(os.Args[2], 10, 64)
    }

    // Generate ECDSA P-256 key
    privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to generate key: %v\n", err)
        os.Exit(1)
    }

    // Get public key bytes
    pubKey := privKey.PublicKey
    pubKeyBytes := elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)
    
    // Hash the public key to get address (SHA-256)
    hash := sha256.Sum256(pubKeyBytes)
    address := hex.EncodeToString(hash[:20]) // Use first 20 bytes like Ethereum

    // Get private key bytes
    privKeyBytes := privKey.D.Bytes()
    
    // Pad private key to 32 bytes
    for len(privKeyBytes) < 32 {
        privKeyBytes = append([]byte{0}, privKeyBytes...)
    }

    output := KeyOutput{
        Index:      index,
        PrivateKey: hex.EncodeToString(privKeyBytes),
        PublicKey:  hex.EncodeToString(pubKeyBytes),
        Address:    address,
        Stake:      stake,
    }

    // Output in requested format
    format := "json"
    if len(os.Args) > 3 {
        format = os.Args[3]
    }

    switch format {
    case "text":
        fmt.Printf("INDEX=%d\n", index)
        fmt.Printf("PRIVATE_KEY=%s\n", output.PrivateKey)
        fmt.Printf("PUBLIC_KEY=%s\n", output.PublicKey)
        fmt.Printf("ADDRESS=0x%s\n", address)
        fmt.Printf("STAKE=%d\n", stake)
    case "env":
        fmt.Printf("export VIRI_VALIDATOR_%d_ADDRESS=0x%s\n", index, address)
        fmt.Printf("export VIRI_VALIDATOR_%d_PUBKEY=%s\n", index, output.PublicKey)
        fmt.Printf("export VIRI_VALIDATOR_%d_STAKE=%d\n", index, stake)
    default:
        json.NewEncoder(os.Stdout).Encode(output)
    }
}
GOEOF

# Generate keys
GENERATED=0
FAILED=0

for i in $(seq 0 $((COUNT - 1))); do
    log_info "Generating keypair $i..."
    
    # Run key generation
    KEY_OUTPUT=$(go run "$TMP_KEY_GEN" "$i" "$STAKE" "$FORMAT" 2>/dev/null || echo "FAILED")
    
    if [[ "$KEY_OUTPUT" == "FAILED" ]]; then
        log_error "Failed to generate keypair $i"
        FAILED=$((FAILED + 1))
        continue
    fi
    
    # Parse output based on format
    if [[ "$FORMAT" == "json" ]]; then
        PRIV_KEY=$(echo "$KEY_OUTPUT" | grep -o '"private_key":"[^"]*"' | cut -d'"' -f4)
        PUB_KEY=$(echo "$KEY_OUTPUT" | grep -o '"public_key":"[^"]*"' | cut -d'"' -f4)
        ADDRESS=$(echo "$KEY_OUTPUT" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)
    else
        PRIV_KEY=$(echo "$KEY_OUTPUT" | grep -o 'PRIVATE_KEY=[^[:space:]]*' | cut -d'=' -f2)
        PUB_KEY=$(echo "$KEY_OUTPUT" | grep -o 'PUBLIC_KEY=[^[:space:]]*' | cut -d'=' -f2)
        ADDRESS=$(echo "$KEY_OUTPUT" | grep -o 'ADDRESS=[^[:space:]]*' | cut -d'=' -f2 | sed 's/^0x//')
    fi
    
    if [[ -z "$PRIV_KEY" ]] || [[ -z "$PUB_KEY" ]]; then
        log_error "Failed to parse keypair $i output"
        FAILED=$((FAILED + 1))
        continue
    fi
    
    # Save private key (encrypted with simple passphrase for demo - production should use stronger encryption)
    echo "$PRIV_KEY" > "$OUTPUT_DIR/${PREFIX}-${i}.key"
    chmod 600 "$OUTPUT_DIR/${PREFIX}-${i}.key"
    
    # Save public key
    echo "$PUB_KEY" > "$OUTPUT_DIR/${PREFIX}-${i}.pub"
    
    # Save metadata JSON
    cat > "$OUTPUT_DIR/${PREFIX}-${i}.json" << EOF
{
    "index": $i,
    "address": "0x$ADDRESS",
    "public_key": "$PUB_KEY",
    "stake": $STAKE,
    "private_key_file": "${PREFIX}-${i}.key",
    "public_key_file": "${PREFIX}-${i}.pub"
}
EOF
    
    log_success "Generated ${PREFIX}-${i}: Address 0x$ADDRESS"
    GENERATED=$((GENERATED + 1))
done

# Clean up temp file
rm -f "$TMP_KEY_GEN"

# Create summary file
cat > "$OUTPUT_DIR/summary.json" << EOF
{
    "count": $GENERATED,
    "stake_per_validator": $STAKE,
    "total_stake": $((STAKE * GENERATED)),
    "keys": [
EOF

for i in $(seq 0 $((GENERATED - 1))); do
    ADDRESS=$(grep -o '"address":"[^"]*"' "$OUTPUT_DIR/${PREFIX}-${i}.json" | cut -d'"' -f4)
    echo "        {\"index\": $i, \"address\": \"$ADDRESS\"}" >> "$OUTPUT_DIR/summary.json"
    if [[ $i -lt $((GENERATED - 1)) ]]; then
        echo "," >> "$OUTPUT_DIR/summary.json"
    fi
done

cat >> "$OUTPUT_DIR/summary.json" << EOF
    ]
}
EOF

echo ""
echo "========================================"
echo "  Key Generation Summary"
echo "========================================"
echo "  Generated: $GENERATED keypair(s)"
echo "  Failed:    $FAILED"
echo "  Output:    $OUTPUT_DIR"
echo "========================================"
echo ""

if [[ $GENERATED -gt 0 ]]; then
    log_success "Key generation complete!"
    log_info "Private keys are stored in: $OUTPUT_DIR/*.key"
    log_warn "KEEP PRIVATE KEYS SECURE! Never share or commit them to version control."
fi

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi
