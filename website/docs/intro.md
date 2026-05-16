# Viri Blockchain

Viri is a modular L1+L2+L3 blockchain built with Go. It features HotStuff BFT consensus, EVM compatibility, libp2p networking, secp256k1 cryptography, and account abstraction.

## Quick Facts

- **Consensus**: HotStuff BFT (4-phase, leader rotation, view change)
- **EVM**: Full Ethereum Virtual Machine compatibility
- **Networking**: libp2p with Kademlia DHT and pubsub
- **Cryptography**: secp256k1 (ECDSA), Keccak256, Merkle-Patricia Trie
- **State Storage**: BadgerDB (embedded key-value store)
- **L2 Features**: Account abstraction (ERC-4337), ZK proofs (groth16), MEV, rollups
- **L3 Features**: Bridge, governance, intents, interop, appchain SDK

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│                   L3                        │
│  Bridge · Governance · Intents · Interop    │
├─────────────────────────────────────────────┤
│                   L2                        │
│  EVM · AA · ZK · Privacy · MEV · Rollups   │
├─────────────────────────────────────────────┤
│                   L1                        │
│  HotStuff · libp2p · State · Crypto · SPV   │
└─────────────────────────────────────────────┘
```

## Getting Started

- **[Quickstart](./deployment/quickstart.md)**: Run a local testnet in minutes
- **[Architecture Overview](./architecture/overview.md)**: Understand the system design
- **[API Reference](./api/json-rpc.md)**: JSON-RPC and REST API documentation
- **[Node Operation](./node/configuration.md)**: Run a validator or full node
