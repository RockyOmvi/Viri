#!/bin/sh
echo "--- nonce ---"
curl -s -X POST http://127.0.0.1:8545 -H 'Content-Type: application/json' -H 'X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5' -d '{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["0x3bb1676433a48e338d0b7c632fde91e8d8345b07","latest"],"id":1}'
echo ""
echo "--- faucet balance ---"
curl -s -X POST http://127.0.0.1:8545 -H 'Content-Type: application/json' -H 'X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5' -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0xdb02aaecf33fcb5d10b0e4eaf77ce04dae67890f","latest"],"id":1}'
echo ""
echo "--- validator balance ---"
curl -s -X POST http://127.0.0.1:8545 -H 'Content-Type: application/json' -H 'X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5' -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x3bb1676433a48e338d0b7c632fde91e8d8345b07","latest"],"id":1}'
echo ""
echo "--- tx receipt ---"
curl -s -X POST http://127.0.0.1:8545 -H 'Content-Type: application/json' -H 'X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5' -d '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt","params":["0xe8986416a3a300387b06b923f3e3fd8467b5de9dfbdaebd17ca6f97a84cec647"],"id":1}'
echo ""
echo "--- latest block ---"
curl -s -X POST http://127.0.0.1:8545 -H 'Content-Type: application/json' -H 'X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5' -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
