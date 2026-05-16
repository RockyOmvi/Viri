# Development Setup

## Prerequisites

- Go 1.25+
- Node.js 20+ (for frontend)
- Docker (for testnet)

## Clone and Build

```bash
git clone https://github.com/viri-chain/viri.git
cd viri

# Build all Go binaries
go build ./...

# Run all tests
go test ./...

# Verify code
go vet ./...
```

## Project Structure

```
├── cmd/
│   ├── virid/          # Main node daemon
│   ├── virictl/        # CLI control tool
│   ├── explorer/       # Embedded explorer
│   ├── faucet/         # Token faucet
│   ├── demo/           # Demo scripts
│   ├── indexer/        # MongoDB indexer
│   └── test_*/         # Test utilities
├── internal/
│   ├── layer1/         # Core protocol
│   │   ├── consensus/  # HotStuff BFT
│   │   ├── p2p/        # libp2p networking
│   │   ├── state/      # State management
│   │   ├── ledger/     # Block/tx/receipt
│   │   ├── crypto/     # secp256k1/Keccak256
│   │   └── sync/       # Block sync
│   ├── layer2/         # Execution layer
│   │   ├── execution/  # EVM engine
│   │   ├── accounts/   # Account abstraction
│   │   ├── zk/         # ZK proofs
│   │   └── vm/         # Virtual machine
│   └── layer3/         # Interop layer
│       ├── bridge/     # Cross-chain bridge
│       ├── governance/ # On-chain governance
│       └── interop/    # Cross-layer comms
├── pkg/sdk/            # Go SDK
├── frontend/           # Next.js explorer
├── deploy/             # Deployment scripts
└── tests/              # Integration tests
```

## Running the Testnet Locally

```bash
docker compose build --no-cache
docker compose up -d
```

## Frontend Development

```bash
cd frontend
npm install
npm run dev
```
