# Validator Operation

## Becoming a Validator

1. Generate a secp256k1 key pair
2. Stake sufficient VIRI tokens (minimum 1,000,000)
3. Configure your node as a validator
4. Connect to the network
5. Participate in consensus

## Requirements

- **Hardware**: 2 vCPU, 4GB RAM, 20GB storage
- **Network**: Static public IP, open ports (30303/TCP for P2P, 8545/TCP for RPC)
- **Uptime**: 24/7 operation recommended

## Consensus Participation

Validators participate in HotStuff BFT consensus:

1. Receive block proposals from the leader
2. Validate and vote on proposals
3. Broadcast votes to peers
4. Trigger view changes when leader fails

## Rewards and Slashing

- **Block Rewards**: Validators earn VIRI for proposing blocks
- **Staking Rewards**: Staked VIRI earns proportional rewards
- **Slashing**: Misbehavior (double signing, downtime) results in stake penalties

## Monitoring

```bash
# Check validator status
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_nodeInfo","params":[],"id":1}'

# Check consensus state
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_getConsensusState","params":[],"id":1}'

# Check connected peers
curl http://localhost:8545 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_getPeers","params":[],"id":1}'
```
