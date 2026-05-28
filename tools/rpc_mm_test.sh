#!/bin/sh
cat > /tmp/j1.json << 'JSON'
{"jsonrpc":"2.0","method":"eth_getBalance","params":["0xdb02aaecf33fcb5d10b0e4eaf77ce04dae67890f","latest"],"id":1}
JSON
cat > /tmp/j2.json << 'JSON2'
{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}
JSON2
cat > /tmp/j3.json << 'JSON3'
{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}
JSON3
cat > /tmp/j4.json << 'JSON4'
{"jsonrpc":"2.0","method":"eth_gasPrice","params":[],"id":1}
JSON4
cat > /tmp/j5.json << 'JSON5'
{"jsonrpc":"2.0","method":"eth_estimateGas","params":[{"from":"0x3bb1676433a48e338d0b7c632fde91e8d8345b07","to":"0xdb02aaecf33fcb5d10b0e4eaf77ce04dae67890f","value":"0x64"}],"id":1}
JSON5

H="Content-Type: application/json"
K="X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5"

echo "=== eth_getBalance ==="
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/j1.json
echo ""
echo "=== eth_chainId ==="
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/j2.json
echo ""
echo "=== net_version ==="
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/j3.json
echo ""
echo "=== eth_gasPrice ==="
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/j4.json
echo ""
echo "=== eth_estimateGas ==="
curl -s -X POST http://127.0.0.1:8545 -H "$H" -H "$K" -d @/tmp/j5.json
echo ""
