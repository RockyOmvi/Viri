# Troubleshooting

## Block Production Stalled

If the network stops producing blocks:

1. **Check validator connectivity**: Ensure all validators have `min_peers` connections
2. **Verify genesis addresses**: Validate that genesis validator addresses match key files
3. **Check leader rotation**: Monitor view changes in logs
4. **Reset testnet**: `docker compose down -v && docker compose up -d`

## Peer Discovery Issues

If nodes can't find peers:

1. **Check API key**: Ensure `api_key.txt` exists and is valid
2. **Increase retries**: The peer-discovery script retries up to 60 times
3. **Verify bootstrap address**: Confirm the bootstrap multiaddress is correct
4. **Open firewall ports**: Ensure port 30303/TCP is accessible

## Consensus Errors

```bash
# Check consensus state
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_getConsensusState","params":[],"id":1}'

# Check validator status
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_nodeInfo","params":[],"id":1}'
```

## Common Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| No blocks produced | Validator addresses mismatch | Regenerate genesis with correct keys |
| Peers not connecting | Firewall or API key issue | Check ports and API key auth |
| Node won't start | Config error | Validate config JSON format |
| High memory usage | State pruning not active | Enable StatePruner in config |
| Slow sync | Network congestion | Check peer count and latency |
| RPC errors | Rate limiting | Reduce request frequency or whitelist IP |
