# P2P Networking

Viri uses **libp2p** for all peer-to-peer communication.

## Peer Discovery

- **Bootstrap Nodes**: Pre-configured entry points for new nodes
- **Kademlia DHT**: Distributed hash table for peer discovery
- **Peer Exchange**: Direct peer exchange between connected nodes

## Messaging

- **GossipSub**: Block and transaction propagation
- **Direct Messaging**: Consensus messages between validators
- **Rate Limiting**: Per-peer message rate limiting to prevent spam

## Peer Management

- **Peer Scoring**: Track peer reliability and behavior
- **Connection Manager**: Maintain optimal number of connections
- **NAT Traversal**: Support for nodes behind NAT/firewalls
- **Auto-relay**: Relay connections for nodes with limited connectivity

## Security

- **Message Signing**: All messages signed with secp256k1
- **API Key Auth**: Optional authentication for RPC/API endpoints
- **Rate Limiting**: Configurable request rate limits
- **IP Whitelisting**: Restrict access to trusted peers
