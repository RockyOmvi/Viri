# Architecture Overview

Viri is a modular blockchain with three layers (L1, L2, L3), each responsible for a distinct set of concerns.

## Layer 1 (Core Protocol)

The L1 layer provides the foundation: consensus, networking, state management, and cryptography.

### Components

- **Consensus Engine**: HotStuff BFT — 4-phase protocol (prepare, pre-commit, commit, decide) with leader rotation and view change
- **P2P Network**: libp2p-based with Kademlia DHT for peer discovery, pubsub for block propagation, and GossipSub for messaging
- **State Management**: Merkle-Patricia Trie for state roots, BadgerDB for persistent key-value storage
- **Ledger**: Block production, transaction indexing, receipt storage
- **Cryptography**: secp256k1 for signatures, Keccak256 for hashing, ECDSA for address derivation
- **Recovery**: Checkpoint-based state recovery, fork resolution, rollback support

### Data Flow

```
Transaction → Mempool → Block Proposal → Consensus → State Update → Storage
                    ↑                          ↓
               P2P Gossip                 Block Finalization
```

## Layer 2 (Execution)

The L2 layer handles transaction execution, smart contracts, and advanced features.

### Components

- **EVM Engine**: Full Ethereum Virtual Machine with all opcodes and precompiles
- **Gas**: EIP-1559 fee model with base fee and priority fee
- **Account Abstraction**: ERC-4337 user operations with entry point contract
- **ZK Proofs**: gnark-based groth16 zero-knowledge proofs for privacy
- **MEV**: Maximal extractable value tracking and mitigation
- **Rollups**: Optimistic and ZK rollup support
- **Agents**: Autonomous agent execution framework

## Layer 3 (Interop)

The L3 layer provides cross-chain interoperability, governance, and application infrastructure.

### Components

- **Bridge**: Cross-chain asset transfer and message passing
- **Governance**: On-chain proposal and voting
- **Intents**: Intent-based transaction execution
- **Interop**: Cross-layer communication protocol
- **Appchain SDK**: Framework for building application-specific chains
- **API**: External API layer for dApps and services
