#!/usr/bin/env bash
# Viri Blockchain - Automated Genesis Ceremony
# Creates N validator keys + runs full ceremony to produce genesis.json
#
# Usage: ./genesis-ceremony.sh [OPTIONS]
#   --validators N       Number of validators (default: 4)
#   --chain-id ID        Chain ID (default: 1337)
#   --network NAME       Network name (default: "viri-testnet")
#   --stake AMOUNT       Stake per validator (default: 1000000)
#   --dir PATH           Ceremony directory (default: ./.viri/genesis)
#   --passphrase PASS    Key passphrase (default: auto-generated)
#   --output FILE        Output genesis file path
#   --help               Show this help

set -euo pipefail

VALIDATORS=4
CHAIN_ID=1337
NETWORK="viri-testnet"
STAKE=1000000
CEREMONY_DIR=".viri/genesis"
PASSPHRASE=""
OUTPUT_FILE=""

usage() {
    echo "Viri Blockchain - Automated Genesis Ceremony"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo "  --validators N       Number of validators (default: 4)"
    echo "  --chain-id ID        Chain ID (default: 1337)"
    echo "  --network NAME       Network name (default: viri-testnet)"
    echo "  --stake AMOUNT       Stake per validator (default: 1000000)"
    echo "  --dir PATH           Ceremony directory (default: .viri/genesis)"
    echo "  --passphrase PASS    Key passphrase (default: auto-generated)"
    echo "  --output FILE        Output genesis file path"
    echo "  --help               Show this help"
    exit 0
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --validators) VALIDATORS="$2"; shift 2 ;;
        --chain-id) CHAIN_ID="$2"; shift 2 ;;
        --network) NETWORK="$2"; shift 2 ;;
        --stake) STAKE="$2"; shift 2 ;;
        --dir) CEREMONY_DIR="$2"; shift 2 ;;
        --passphrase) PASSPHRASE="$2"; shift 2 ;;
        --output) OUTPUT_FILE="$2"; shift 2 ;;
        --help|-h) usage ;;
        *) echo "Unknown: $1"; usage ;;
    esac
done

if ! command -v virictl &> /dev/null; then
    if [[ -f "./virictl" ]]; then
        VIRICTL="./virictl"
    elif [[ -f "./virictl.exe" ]]; then
        VIRICTL="./virictl.exe"
    else
        echo "Building virictl..."
        go build -o virictl ./cmd/virictl
        VIRICTL="./virictl"
    fi
else
    VIRICTL="virictl"
fi

rm -rf "$CEREMONY_DIR"

echo "=== Initializing genesis ceremony ==="
$VIRICTL genesis init --dir "$CEREMONY_DIR" --chain-id "$CHAIN_ID" --network "$NETWORK"

if [[ -z "$PASSPHRASE" ]]; then
    PASSPHRASE=$(openssl rand -hex 32)
    echo "Auto-generated passphrase: $PASSPHRASE"
fi

mkdir -p "$CEREMONY_DIR/keys"

echo ""
echo "=== Generating $VALIDATORS validator keys ==="
for i in $(seq 0 $((VALIDATORS - 1))); do
    echo "  Generating key for validator-$i..."

    KEY_FILE="$CEREMONY_DIR/keys/validator-$i.key"
    TMP_SCRIPT=$(mktemp /tmp/viri-keygen-XXXXXX.go)
    cat > "$TMP_SCRIPT" << 'GOEOF'
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
)

type KeyOutput struct {
    PrivateKey string `json:"private_key"`
    PublicKey  string `json:"public_key"`
    Address    string `json:"address"`
}

func main() {
    privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        fmt.Fprintf(os.Stderr, "gen key: %v\n", err)
        os.Exit(1)
    }
    pubKeyBytes := elliptic.Marshal(privKey.PublicKey.Curve, privKey.PublicKey.X, privKey.PublicKey.Y)
    addrHash := sha256.Sum256(pubKeyBytes)
    output := KeyOutput{
        PrivateKey: hex.EncodeToString(privKey.D.Bytes()),
        PublicKey:  hex.EncodeToString(pubKeyBytes),
        Address:    "0x" + hex.EncodeToString(addrHash[:20]),
    }
    json.NewEncoder(os.Stdout).Encode(output)
}
GOEOF

    KEY_OUTPUT=$(go run "$TMP_SCRIPT" 2>/dev/null || true)
    rm -f "$TMP_SCRIPT"

    if [[ -z "$KEY_OUTPUT" ]]; then
        echo "  FAILED to generate key for validator-$i"
        exit 1
    fi

    PRIV_KEY=$(echo "$KEY_OUTPUT" | grep -o '"private_key":"[^"]*"' | cut -d'"' -f4)
    PUB_KEY=$(echo "$KEY_OUTPUT" | grep -o '"public_key":"[^"]*"' | cut -d'"' -f4)

    echo "$PRIV_KEY" > "$KEY_FILE"

    $VIRICTL genesis add-validator --dir "$CEREMONY_DIR" \
        --name "validator-$i" \
        --pubkey "$PUB_KEY" \
        --stake "$STAKE"
done

echo ""
echo "=== Signing manifest ==="
for i in $(seq 0 $((VALIDATORS - 1))); do
    KEY_FILE="$CEREMONY_DIR/keys/validator-$i.key"
    echo "  Signing with validator-$i..."
    $VIRICTL genesis sign --dir "$CEREMONY_DIR" --key "$KEY_FILE" --passphrase "$PASSPHRASE"
done

echo ""
echo "=== Verifying ==="
$VIRICTL genesis verify --dir "$CEREMONY_DIR"

echo ""
echo "=== Finalizing ==="
$VIRICTL genesis finalize --dir "$CEREMONY_DIR"

echo ""
echo "=== Genesis file ==="
GENESIS_FILE="$CEREMONY_DIR/genesis.json"

if [[ -n "$OUTPUT_FILE" ]]; then
    cp "$GENESIS_FILE" "$OUTPUT_FILE"
    echo "  Copied to: $OUTPUT_FILE"
fi

echo "  Chain ID:       $CHAIN_ID"
echo "  Network:        $NETWORK"
echo "  Validators:     $VALIDATORS"
echo "  Total stake:    $((STAKE * VALIDATORS))"
echo "  Genesis file:   $GENESIS_FILE"
echo "  Passphrase:     $PASSPHRASE"
echo ""
echo "IMPORTANT: Keep all key files in $CEREMONY_DIR/keys/ secure!"
echo "Distribute genesis.json to all testnet participants."
