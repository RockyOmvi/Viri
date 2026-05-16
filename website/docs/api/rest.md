# REST API

Viri provides REST API endpoints at port 8546 (configurable).

## Endpoints

### GET /api/v1/status
Network status overview.

```json
{
  "version": "0.1.0",
  "height": 1000,
  "tip": "0x...",
  "peers": 4,
  "blocks_in": 500,
  "blocks_out": 500,
  "txs_in": 2000,
  "txs_out": 2000,
  "uptime": "10m30s"
}
```

### GET /api/v1/blocks
Recent blocks list.

Query params: `?from=N&to=N&limit=N`

### GET `/api/v1/blocks/{height}`
Single block by height.

### GET `/api/v1/transactions/{hash}`
Transaction details by hash.

### GET `/api/v1/accounts/{address}`
Account information.

```json
{
  "address": "0x...",
  "balance": "1000000",
  "nonce": 5,
  "type": 0,
  "has_code": false
}
```

### GET /api/v1/peers
Connected peers list.

### GET /api/v1/health
Health check endpoint.

### POST /api/v1/faucet/claim
Claim testnet tokens.

```json
{"address": "0x..."}

// Response
{"success": true, "txHash": "0x..."}
```

### GET /api/v1/faucet/info
Faucet configuration and balance.

## Indexer API (port 8547)

### GET `/api/v1/address/{address}/transactions`
Transaction history for an address.

Query params: `?page=1&limit=20`

### GET /api/v1/blocks
Paginated block list.

Query params: `?page=1&limit=20`

### GET /api/v1/status
Indexer sync status.
