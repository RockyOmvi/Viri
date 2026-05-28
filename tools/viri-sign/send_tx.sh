#!/bin/sh
TX_HEX=$(cat /tmp/tx.hex | tr -d '[:space:]')
curl -s -X POST http://127.0.0.1:8545 \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: fa776f1924b0a52cac9c8857cc9743723cd8c51541e896cc2dd9b534ccbe4df5' \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendRawTransaction\",\"params\":[\"0x${TX_HEX}\"],\"id\":1}"
