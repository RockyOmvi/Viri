#!/bin/sh
cat > /tmp/rpc_block.json << 'JSONEOF'
{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}
JSONEOF

cat > /tmp/rpc_balance.json << 'JSONEOF'
{"jsonrpc":"2.0","method":"eth_getBalance","params":["0xdb02aaecf33fcb5d10b0e4eaf77ce04dae67890f","latest"],"id":1}
JSONEOF

cat > /tmp/rpc_nonce.json << 'JSONEOF'
{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["0x3bb1676433a48e338d0b7c632fde91e8d8345b07","latest"],"id":1}
JSONEOF

H="Content-Type: application/json"
K="X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5"

echo "=== SERVICES VM (localhost:8545) ==="
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/rpc_block.json
echo ""
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/rpc_balance.json
echo ""
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/rpc_nonce.json
echo ""

echo "=== BOOTSTRAP VM (10.0.1.12:8545) ==="
curl -s -X POST http://10.0.1.12:8545 -H "$H" -H "$K" -d @/tmp/rpc_block.json
echo ""
curl -s -X POST http://10.0.1.12:8545 -H "$H" -H "$K" -d @/tmp/rpc_balance.json
echo ""

echo "=== VIA NGINX (localhost/rpc) ==="
curl -s -X POST http://127.0.0.1/rpc -H "$H" -H "$K" -d @/tmp/rpc_block.json
echo ""
curl -s -X POST http://127.0.0.1/rpc -H "$H" -H "$K" -d @/tmp/rpc_balance.json
echo ""
