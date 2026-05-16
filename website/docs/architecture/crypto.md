# Cryptography

## Key Types

- **secp256k1**: All transaction signing and consensus messages
- **ECDSA**: Digital signature algorithm (Ethereum-compatible)
- **Keccak256**: Hash function for addresses and state roots

## Address Derivation

```
PublicKey (64 bytes uncompressed, no 0x04 prefix)
    → Keccak256(publicKey)
    → Last 20 bytes → Address (0x-prefixed hex)
```

## Signature Scheme

1. Hash the message with Keccak256
2. Sign with secp256k1 private key
3. Signature: `(R, S, V)` where V is recovery ID

## ZK Proofs

- **gnark**: Go-based zk-SNARK library
- **groth16**: Zero-knowledge proof system
- **Circuit Caching**: Proving and verifying keys cached for performance

## TLS

TLS certificates for API servers use P-256 (NIST) since Go's `crypto/x509` does not support secp256k1.
