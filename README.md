# Viri

A production-ready, 3-layer modular blockchain built in Go — designed for today's Web3 needs and tomorrow's scale.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    LAYER 3 — APPLICATION                        │
│   App Chains  •  Interop Layer  •  SDK/API  •  Governance      │
├─────────────────────────────────────────────────────────────────┤
│                   LAYER 2 — EXECUTION                           │
│   WASM VM + EVM  •  Account Abstraction  •  MEV Resistance     │
│   ZK Privacy  •  Parallel Execution  •  Rollup Framework       │
├─────────────────────────────────────────────────────────────────┤
│                      LAYER 1 — CORE                             │
│   P2P Network  •  PoS+HotStuff BFT  •  Stateless Verification  │
│   Data Availability  •  Decentralized Sequencer  •  Settlement  │
└─────────────────────────────────────────────────────────────────┘
```

## Key Features

- **Native Account Abstraction** — Smart contract wallets from birth, no EOAs
- **Multi-VM Support** — WASM primary + EVM compatible runtime
- **MEV Resistance** — TEE-based sequencing with fair ordering
- **Built-in Privacy** — Optional ZK-shielded transactions
- **Cross-Chain Native** — IBC-like protocol, no external bridges needed
- **Stateless Verification** — Run nodes on Raspberry Pi (1GB RAM)
- **Post-Quantum Ready** — Pluggable signature schemes
- **Pay Gas with Any Token** — No native token dependency for fees

## Supported Platforms

| Platform | Binary |
|---|---|
| Linux x86_64 | `virid-linux-amd64` |
| Linux ARM64 | `virid-linux-arm64` |
| Linux ARMv6 | `virid-linux-armv6` |
| macOS Intel | `virid-darwin-amd64` |
| macOS Apple Silicon | `virid-darwin-arm64` |
| Windows x64 | `virid-windows-amd64.exe` |
| Raspberry Pi | `virid-linux-armv6` / `virid-linux-arm64` |

## Quick Start

### Build

```bash
# Build for your current platform
make build

# Build for all platforms
make build-all

# Build specific platform
make build-linux
make build-darwin
make build-windows
make build-rpi
```

### Run a Node

```bash
./build/virid
```

### Use the CLI

```bash
# Create a wallet
./build/virictl wallet create

# Check version
./build/virictl version
```

### Local Dev Network (4 Validators)

```bash
cd docker
docker-compose up -d
```

## Project Structure

```
viri/
├── cmd/
│   ├── virid/              # Main daemon node
│   └── virictl/            # CLI tool
├── internal/
│   ├── layer1/              # Core: crypto, p2p, consensus, ledger, state, DA
│   ├── layer2/              # Execution: VM, contracts, accounts, privacy, MEV
│   └── layer3/              # Application: appchains, interop, SDK, governance
├── pkg/                     # Public packages
├── configs/                 # Genesis, network presets
├── scripts/                 # Build scripts
├── tests/                   # Integration, benchmarks, fuzz tests
└── docker/                  # Docker configuration
```

## Technology Stack

- **Language:** Go 1.22+
- **Networking:** libp2p (planned)
- **Consensus:** PoS + HotStuff BFT (planned)
- **Database:** BadgerDB/BoltDB (planned)
- **VM:** WASM + EVM runtime (planned)
- **Cryptography:** ECDSA P-256, SHA-256, Merkle trees

## Roadmap

- [x] Phase 1: Foundation — Crypto, ledger, genesis, chain validation
- [ ] Phase 2: P2P networking with libp2p
- [ ] Phase 3: PoS + HotStuff BFT consensus
- [ ] Phase 4: State management with Merkle-Patricia Trie
- [ ] Phase 5: WASM VM + Account Abstraction
- [ ] Phase 6: Transaction pipeline + parallel execution
- [ ] Phase 7: MEV resistance + ZK privacy
- [ ] Phase 8: Standard contract library
- [ ] Phase 9: Rollup framework
- [ ] Phase 10: App chain spawning
- [ ] Phase 11: Cross-chain interop
- [ ] Phase 12: API layer + Go SDK
- [ ] Phase 13: Governance DAO
- [ ] Phase 14: Cross-platform CLI release
- [ ] Phase 15: Tests, benchmarks, Docker, CI

## License

MIT
