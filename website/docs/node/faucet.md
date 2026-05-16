# Faucet Operation

The faucet distributes testnet VIRI tokens to users.

## Configuration

Run the faucet with a dedicated funded account:

```bash
virid faucet \
  --faucet-key <hex-private-key> \
  --api-addr :8546 \
  --domain faucet.testnet.viri.me
```

## Endpoints

### GET /api/v1/faucet/info
Returns faucet configuration:

```json
{
  "perClaim": 10,
  "dailyLimit": 50,
  "cooldownHours": 24,
  "balance": 10000
}
```

### POST /api/v1/faucet/claim
Claim tokens:

```json
// Request
{"address": "0x..."}
// Response
{"success": true, "txHash": "0x..."}
```

## Rate Limiting

- **Per Address**: 10 VIRI per 24 hours
- **Global**: Configurable daily limit
- **IP-based**: Rate limiting per IP

## Fund Management

Monitor the faucet balance and refill as needed:

```bash
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["<faucet-address>","latest"],"id":1}'
```
