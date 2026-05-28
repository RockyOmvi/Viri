# WebSocket API

Viri supports WebSocket connections for real-time event streaming.

## Connection

```
ws://localhost:8547
wss://localhost:8547
```

The WebSocket port is `rpc_port + 2` (default 8547).

## Authentication (Optional)

If the server requires API key authentication, pass it as a query parameter:

```
ws://localhost:8547?api_key=your-api-key-here
```

Or send an auth message on connect:

```json
{"type": "auth", "apiKey": "your-api-key-here"}
```

## Subscribe

Subscribe to a channel by sending:

```json
{"type": "subscribe", "channel": "new_blocks"}
```

Available channels:

| Channel | Description |
|---------|-------------|
| `new_blocks` | New blocks as they are produced |
| `new_transactions` | New pending transactions |
| `peers` | Peer connect/disconnect events |

### Response

```json
{"type": "subscribed", "channel": "new_blocks"}
```

### Event Notification

```json
{
  "type": "event",
  "channel": "new_blocks",
  "data": {
    "number": "0x1b5",
    "hash": "0x...",
    "parentHash": "0x...",
    "timestamp": "0x65f0180b",
    "transactionCount": 3,
    "miner": "0x..."
  }
}
```

## Unsubscribe

```json
{"type": "unsubscribe", "channel": "new_blocks"}
```

Response:

```json
{"type": "unsubscribed", "channel": "new_blocks"}
```
