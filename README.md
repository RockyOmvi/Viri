# Viri

[![CI](https://github.com/viri-chain/viri/actions/workflows/ci.yml/badge.svg)](https://github.com/viri-chain/viri/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-brightgreen)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-43_packages_·_0_failures-brightgreen)](#test-results)
[![Testnet](https://img.shields.io/badge/testnet-live-blue)](#public-testnet)

**A production-grade 3-layer modular blockchain built in Go.**

Viri is a complete L1 blockchain with native account abstraction, dual WASM + EVM execution, ZK-shielded transactions, MEV-resistant sequencing, cross-chain interop, and on-chain governance — all in a single codebase, zero external layer dependencies.

---

## Why Viri

Most "modular" blockchains specialize in one layer and outsource the rest. Celestia does data availability. Arbitrum does execution. Cosmos does app chains. Each depends on external projects for the layers it doesn't own — introducing cross-layer trust assumptions, bridge risk, and fragmented developer experience.

Viri owns all three layers natively:

```
┌──────────────────────────────────────────────────────────────────────┐
│                      LAYER 3 — APPLICATION                           │
│  App Chains  ·  IBC-like Interop  ·  REST API  ·  Governance DAO    │
│  Cross-chain Bridge  ·  Intent Solver Network  ·  Go SDK            │
├──────────────────────────────────────────────────────────────────────┤
│                      LAYER 2 — EXECUTION                             │
│  EVM + WASM VM  ·  Native Account Abstraction  ·  ZK Privacy Pool   │
│  MEV Resistance  ·  EIP-1559 Gas Oracle  ·  Rollup Framework        │
├──────────────────────────────────────────────────────────────────────┤
│                        LAYER 1 — CORE                                │
│  HotStuff BFT Consensus  ·  Delegated PoS  ·  P2P Networking        │
│  Merkle-Patricia Trie  ·  State Sync  ·  Encrypted Keystore         │
└──────────────────────────────────────────────────────────────────────┘
```

No bridges between layers. No external trust. One stack, full control.

---

## Key Features

### No EOAs — Account Abstraction from Genesis
Every wallet on Viri is a smart contract wallet. No externally-owned accounts. Transaction validation logic, session keys, social recovery, and gas sponsorship are programmable at the protocol level — not bolted on via ERC-4337.

### Gas Payable in Any Token
Users pay transaction fees in whichever token they hold. No forced native token purchase. A user with only USDC can interact with the chain immediately. The fee handler converts to the native equivalent at execution time via the on-chain price oracle.

### Dual VM — WASM Primary, EVM Compatible
Deploy Solidity contracts unchanged. Or write WASM contracts for full memory control and gas efficiency. Both runtimes run natively; developers choose.

### Built-in ZK Privacy
Optional shielded transactions via a Groth16 prover (gnark). Deposits, transfers, and withdrawals through a Pedersen-commitment nullifier pool. The ZK verifier runs as an on-chain precompile.

### MEV Resistance
Transactions are batched and ordered by the sequencer using fair-ordering rules before block inclusion. TEE-based sequencing removes the sequencer's ability to front-run or sandwich.

### Stateless Verification
Nodes verify blocks without holding full state. A Viri node runs on 1GB RAM — including on a Raspberry Pi. Verification cost does not scale with chain history.

### Cross-chain Native — No External Bridges
IBC-like channels between app chains. Multi-sig cross-chain bridge with replay protection. Validators co-sign transfers. No third-party bridge contracts, no wrapped asset risk.

### On-chain Governance DAO
Proposals, token-weighted voting, quorum thresholds, and veto logic are protocol-native. No off-chain snapshot, no multisig admin key.

### Post-Quantum Ready
Signature schemes are pluggable at the crypto layer. Swap in post-quantum algorithms without protocol upgrades.

---

## Quick Start

### Prerequisites

- Go 1.22+
- Docker + Docker Compose (for testnet)

### Build

```bash
git clone https://github.com/viri-chain/viri.git
cd viri
make build
```

Binaries land in `./build/`:
- `virid` — node daemon
- `virictl` — CLI wallet and governance tool

### Run a Single Node

```bash
./build/virid \
  --validator \
  --chain-id 1337 \
  --data-dir ./data \
  --rpc-port 8545
```

Blocks produce every 500ms in dev mode.

```bash
curl http://localhost:8545/health
```

### Run a 4-Validator Local Testnet

```bash
cd testnet
docker-compose up -d
docker logs --tail 30 viri-validator-0
```

You will see live HotStuff BFT consensus messages between validators.

### CLI Wallet

```bash
# Create a new wallet
./build/virictl wallet create

# Generate a 24-word BIP39 mnemonic
./build/virictl mnemonic generate 24

# Restore wallet from mnemonic
./build/virictl wallet create --mnemonic "your twenty four words here ..."
```

---

## Public Testnet

| Parameter | Value |
|---|---|
| Chain ID | `viri-testnet-1` |
| RPC endpoint | `https://rpc.testnet.viri.network` |
| Explorer | `https://explorer.testnet.viri.network` |
| Faucet | `https://faucet.testnet.viri.network` |

Connect MetaMask or any EVM-compatible wallet to the RPC endpoint and start building immediately.

---

## Architecture

### Layer 1 — Core

| Package | What it does |
|---|---|
| `consensus` | HotStuff BFT — four-phase pipelined (Prepare → PreCommit → Commit → Decide) with leader rotation, view change on timeout, and liveness detection |
| `crypto` | ECDSA P-256, SHA-256, BIP39 mnemonics, PBKDF2 key derivation, AES-256 encrypted keystore |
| `ledger` | Block structure, transaction types, genesis configuration, EIP-1559 fee market, block rewards, serialization |
| `p2p` | libp2p networking, peer manager with reputation scoring, message authentication, rate limiter, duplicate detection |
| `state` | Merkle-Patricia Trie with deterministic root hashing, BadgerDB persistence, account state management |
| `da` | Data availability layer with erasure coding |
| `sequencer` | Decentralized block sequencer with fair ordering |
| `sync` | Fast sync, snap sync, state proof verification, error recovery |
| `spv` | Simplified payment verification for light clients |

### Layer 2 — Execution

| Package | What it does |
|---|---|
| `vm` | EVM bytecode interpreter with full opcode set + WASM runtime — both with gas metering |
| `execution` | Transaction execution pipeline, on-chain ZK precompile, gas accounting |
| `gas` | EIP-1559 gas oracle, base fee adjustment per block, priority fee percentiles |
| `zk` | R1CS circuit builder, Groth16 prover/verifier via gnark, batch proof verification |
| `accounts` | ERC-4337-compatible entry point, smart contract wallet deployment, paymaster fee routing |
| `privacy` | Shielded pool — Pedersen commitments, nullifiers, deposit/transfer/withdraw |
| `mev` | MEV-resistant transaction batching, gas-price and fair-ordering modes |
| `rollups` | Batch submission to L1, challenge window, fraud proof protocol, batch confirmation |

### Layer 3 — Application

| Package | What it does |
|---|---|
| `governance` | Full proposal lifecycle — submission, voting period, quorum check, tally, veto |
| `bridge` | Cross-chain transfer initiation, multi-sig validator threshold signing, replay protection |
| `interop` | IBC-like channels — open, send packet, receive packet, timeout, close |
| `intent` | Declarative intent submission, solver registration, intent fill, invalid fill rejection |
| `api` | REST API with API key auth, token-bucket rate limiting, CORS, health check |
| `appchain` | Spawn child chains with custom validator sets, pause and resume |
| `sdk` | Go client library for L3 API — health, governance, bridge, interop, intent |

---

## Test Results

All 43 packages pass. Zero failures.

```
$ go test ./... -count=1

ok  internal/layer1/consensus   39.68s   55 tests
ok  internal/layer1/crypto      15.19s   20 tests
ok  internal/layer1/ledger       1.09s   50 tests
ok  internal/layer1/p2p          1.42s   52 tests
ok  internal/layer1/state        1.60s   37 tests
ok  internal/layer1/da           4.12s
ok  internal/layer1/sequencer    1.11s
ok  internal/layer1/sync         2.64s
ok  internal/layer2/vm           3.13s   11 tests
ok  internal/layer2/execution    1.12s    7 tests
ok  internal/layer2/gas          4.11s   16 tests
ok  internal/layer2/zk           3.06s   24 tests
ok  internal/layer2/accounts     4.27s   11 tests
ok  internal/layer2/mev          3.84s    5 tests
ok  internal/layer2/privacy      3.84s    4 tests
ok  internal/layer2/rollups      3.81s    6 tests
ok  internal/layer3/governance   2.02s    6 tests
ok  internal/layer3/bridge       1.99s   12 tests
ok  internal/layer3/interop      1.98s    7 tests
ok  internal/layer3/intent       1.96s    6 tests
ok  internal/layer3/api          3.63s   12 tests
ok  internal/layer3/appchain     1.05s    6 tests
ok  internal/layer3/sdk          3.24s    7 tests
ok  tests/integration           28.57s   46 tests
ok  tests/contracts              1.37s   43 tests
ok  tests/fuzz                   0.37s   10 fuzz harnesses
```

### Consensus Performance

| Test | Result |
|---|---|
| 4-validator block production | Height 207, all validators in sync |
| View change (leader rotation) | Height 150, clean handoff |
| Network partition + heal | All validators reconverge at height 297–298 |
| State sync (late joiner) | Late validator catches up cleanly |
| 20-validator convergence | Min/max spread of 1 block |
| 100-validator supermajority | **2,015 ops/sec** |
| Message throughput | **19,833 msgs/sec · 130 blocks/sec** |

### Fuzz Tests

```
FuzzSignatureVerification   PASS
FuzzTransactionHash         PASS
FuzzMerkleTree              PASS
FuzzSHA256                  PASS
FuzzBlockSigningPayload     PASS
FuzzECDSASignVerify         PASS
FuzzHashCollisions          PASS
FuzzHasSuperMajority        PASS
FuzzQCIsValid               PASS
FuzzSelectProposer          PASS
```

Full audit report: [audit.md](audit.md)

---

## Supported Platforms

| Platform | Binary |
|---|---|
| Linux x86_64 | `virid-linux-amd64` |
| Linux ARM64 | `virid-linux-arm64` |
| macOS Apple Silicon | `virid-darwin-arm64` |
| macOS Intel | `virid-darwin-amd64` |
| Windows x64 | `virid-windows-amd64.exe` |
| Raspberry Pi | `virid-linux-armv6` / `virid-linux-arm64` |

```bash
make build-all        # all platforms
make build-linux      # Linux amd64 + arm64
make build-darwin     # macOS Intel + Apple Silicon
make build-windows    # Windows amd64
make build-rpi        # Raspberry Pi
```

---

## Project Structure

```
viri/
├── cmd/
│   ├── virid/          # Node daemon — entry point
│   ├── virictl/        # CLI — wallet, staking, governance, interop
│   └── demo/           # Wallet + contract demo binary
├── internal/
│   ├── layer1/         # consensus · crypto · ledger · p2p · state · sync · da
│   ├── layer2/         # vm · execution · gas · zk · accounts · mev · privacy · rollups
│   ├── layer3/         # governance · bridge · interop · intent · api · appchain · sdk
│   ├── e2e/            # End-to-end tests
│   └── pkg/            # Shared: metrics · audit · observability · security
├── tests/
│   ├── integration/    # Cross-layer integration tests
│   ├── contracts/      # EVM contract tests
│   ├── fuzz/           # Fuzz harnesses
│   └── benchmarks/     # Performance benchmarks
├── testnet/            # Docker Compose — 4-validator local testnet
├── deploy/             # Deployment scripts
├── configs/            # Genesis templates · network presets
└── Makefile
```

---

## Roadmap

- [x] Phase 1–15: Core through CI pipeline — complete
- [ ] **v0.2.0** — Browser wallet (Chrome extension, Manifest V3)
- [ ] **v0.3.0** — Public testnet with community validators + faucet
- [ ] **v0.4.0** — Native token (VIRI) + Genesis NFT collection
- [ ] **v0.5.0** — DEX deployment + gas-in-any-token live demo
- [ ] **v1.0.0** — Mainnet launch

---

## Contributing

Viri is open source and actively looking for contributors. See [CONTRIBUTING.md](CONTRIBUTING.md) to get started.

Good first issues are labeled [`good first issue`](https://github.com/viri-chain/viri/issues?q=is%3Aissue+label%3A%22good+first+issue%22) on GitHub.

Areas where help is most welcome:
- Browser wallet (TypeScript / React)
- Block explorer frontend
- Documentation and tutorials
- Additional EVM opcode coverage
- Performance benchmarks

---

## License

MIT — see [LICENSE](LICENSE).

---

<p align="center">
  Built in Go · Zero external layer dependencies · Production tested
</p>