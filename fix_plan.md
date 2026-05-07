# Viri Blockchain — Fix Implementation Plan

## Fixes Being Applied

| # | Flaw | File(s) | Fix |
|---|------|---------|-----|
| 1 | Fake Keccak256 | `crypto/keys.go` | Use `golang.org/x/crypto/sha3` |
| 2 | Signature.Bytes() ambiguous | `crypto/keys.go` | Fixed 32-byte R/S with padding |
| 3 | Unencrypted private key storage | `crypto/keystore.go` (new) + `virid/main.go` | AES-256-GCM + scrypt KDF |
| 4 | Private key printed to console | `virictl/main.go` | Add security warning, save to encrypted file |
| 5 | Wildcard CORS | `virid/api_server.go` | Restrict to localhost |
| 6 | API server graceful shutdown | `virid/api_server.go` | Use `Shutdown()` instead of `Close()` |
| 7 | RPC stub endpoints (getBalance, getTransactionCount) | `virid/rpc_server.go` | Wire to real StateManager |
| 8 | sendRawTransaction no-op | `virid/rpc_server.go` | Decode, validate, add to txpool |
| 9 | Hardcoded chain ID | `virid/rpc_server.go` | Use config chain ID |
| 10 | Hardcoded validator:true | `virid/rpc_server.go` | Use actual config |
| 11 | Fake state root | `state/state.go` | Compute Merkle root from accounts |
| 12 | Broken CLI API URL | `virictl/main.go` | Proper URL parsing |
| 13 | tx send is no-op | `virictl/main.go` | Implement actual RPC submission |
| 14 | createVoteData not deterministic | `consensus/hotstuff.go` | Binary encoding |
| 15 | GetBlocks swallows errors | `ledger/persistent_chain.go` | Return errors |
| 16 | Error swallowed on account creation | `virid/main.go` | Log properly |
| 17 | Nonce off-by-one | `execution/engine.go` | Fix comparison |
| 18 | Contract address collision | `execution/engine.go` | Hash(sender + nonce) |
| 19 | Graceful shutdown for servers | `virid/main.go` | Wire Stop() calls |
| 20 | Block index unbounded | `ledger/persistent_chain.go` | LRU cache |
| 21 | Dockerfile user mismatch | `docker/Dockerfile` | Fix to `viri` |
| 22 | .gitignore for binaries | `.gitignore` (new) | Exclude .exe and build/ |
| 23 | API account/tx endpoints | `virid/api_server.go` | Implement with real data |
| 24 | RPC+API config stored on server | `virid/rpc_server.go` | Add chainID field |
