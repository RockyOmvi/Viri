# Layer 2 (Execution)

## EVM

Viri includes a full **Ethereum Virtual Machine** implementation:

- All EVM opcodes supported
- Precompiled contracts (ec-recover, sha256, ripemd160, etc.)
- EIP-1559 fee model (base fee + priority fee)
- Gas metering and limits

## Account Abstraction (ERC-4337)

- **User Operations**: Alternative to traditional transactions
- **Entry Point**: Smart contract that validates and executes UserOps
- **Paymasters**: Sponsors gas fees for users
- **Smart Wallets**: Programmable wallet contracts

## ZK Proofs

- **groth16**: Zero-knowledge proving system
- **Prover/Verifier**: Real gnark circuit compilation with caching
- **Privacy**: Shielded transactions via ZK proofs

## MEV

- **Tracking**: Monitor maximal extractable value opportunities
- **Mitigation**: Fair ordering and MEV-resistant block production
- **Auction**: Optional block space auction mechanism

## Rollups

- **Optimistic**: Fraud-proof based rollup support
- **ZK**: Validity-proof based rollup support
