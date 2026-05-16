# Changelog

## v0.1.0-testnet (2026-05-13)

First public testnet release. All 43 packages operational.

### Layer 1 — Core

- **consensus** — HotStuff BFT (4-phase pipelined: Prepare, PreCommit, Commit, Decide). Leader rotation, view change, liveness detection. Tested to 100 validators with 2000+ ops/sec supermajority verification. Throughput: 4958 msgs/sec, 130 blocks/sec.
- **crypto** — ECDSA secp256k1 key generation, signing, verification. EIP-2 low-S canonical signatures. Keccak256 address derivation. Encrypted keystore with scrypt key derivation. BIP39 mnemonic support (12/15/18/21/24 words) with full 2048-word English wordlist.
- **ledger** — Block production with fee market (EIP-1559 style base fee adjustment). Genesis validation, block/transaction serialization (binary + JSON). Economics with block rewards, inflation, burned fees.
- **p2p** — libp2p networking with peer manager (add/remove/ban/evict), reputation scoring (trusted/healthy/suspicious/toxic), rate limiter (message + byte), message propagation with TTL, handshake protocol.
- **state** — Merkle-Patricia Trie with proof generation/verification. BadgerDB persistent store. Account state management (balance, nonce, code, storage).
- **sync** — Fast sync (header download → state snapshot → recent blocks). Snap sync with pivot block. Progress reporting, error recovery, completion detection.

### Layer 2 — Execution

- **vm** — EVM bytecode interpreter supporting all standard opcodes (arithmetic, bitwise, memory, storage, calldata, environment, control flow, logging, SHA3, etc.). WASM VM with stack operations, gas metering, register imports.
- **execution** — Transaction execution engine with transfer, contract deploy, contract call. Precompile for on-chain ZK proof verification (gnark Groth16). Gas accounting.
- **gas** — EIP-1559 gas oracle with base fee adjustment (up/down/same). Priority fee percentiles. Network utilization tracking. Block gas limit enforcement.
- **zk** — R1CS circuit builder with arithmetic, boolean, range, MUL constraints. Fiat-Shamir prover/verifier. Groth16-style proof with batched verification. Shielded transaction format with Merkle tree commitments.
- **accounts** — ERC-4337 entry point for account abstraction. Wallet deployment, execution with call data. Paymaster beneficiary fees. Nonce management.
- **privacy** — Shielded pool with Pedersen commitments and nullifiers. Deposit, withdraw, transfer operations. Duplicate commitment and nullifier rejection.
- **mev** — MEV-resistant batching with delay. Transaction ordering by gas price and MEV-optimized strategies.
- **rollups** — Batch submission, challenge and confirm protocol. Metadata accessors.

### Layer 3 — Application

- **governance** — DAO with proposal submission, token-weighted voting (yes/no/veto/abstain), quorum calculation, automatic tally on period end.
- **bridge** — Cross-chain bridge with chain registration, validator signature threshold (2/3), lock-complete lifecycle. Privacy bridge with replay protection, double-spend prevention, transfer pruning.
- **interop** — IBC-like protocol with channel creation (two ports, two chains), packet send/receive with timeout, channel close.
- **intent** — Intent solver network. Submit intent, register solver, solve/fill lifecycle. Invalid state rejection.
- **api** — REST API server with governance, bridge, interop, intent endpoints. API key authentication (header + query string). Token-bucket rate limiting. CORS support.
- **appchain** — App chain creation with custom validator sets. Pause/resume lifecycle.
- **sdk** — Go client library with health check, governance, bridge, interop, intent RPC methods.

### Tests & Infrastructure

- **integration** — 46 tests covering multi-node validator set, governance flow, bridge flow, interop, intent, ZK shielded transactions, gas oracle, full blockchain with governance, network partition, view change, state sync, state persistence, RPC, sync modes.
- **contracts** — 43 EVM opcode tests (all standard opcodes verified with expected outputs).
- **fuzz** — 10 fuzz harnesses: signature verification, transaction hash, Merkle tree, SHA256, block signing, ECDSA, hash collisions, supermajority, QC validation, proposer selection.
- **benchmarks** — Benchmark suite for performance measurement.
- **CI** — GitHub Actions pipeline with test, lint, fuzz, security scan, build (5 platforms), Docker publish.
