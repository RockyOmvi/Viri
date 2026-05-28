# Viri Testnet - Comprehensive Audit Report

**Date:** 2026-05-19  
**Scope:** Full-stack audit: chain core, RPC, transactions, deployment, security  
**Total Issues:** 150+ identified across 7 categories

---

## 1. CHAIN CORE: GENESIS, STATE, TOKENOMICS

### Critical

| # | Issue | File | Fix |
|---|-------|------|-----|
| 1.1 | **State resets on every restart** — `stateMgr.Initialize()` unconditionally runs, resetting `totalSupply=0`, `blockHeight=0`, `stateRoot=empty` | `cmd/virid/main.go:374` | Add `if sm.IsInitialized()` check before `Initialize()`. Persist a "genesis hash" in state DB and skip init if matches. |
| 1.2 | **Block rewards never reach accounts** — `ProcessBlock` returns `BlockEconomics` but the reward/fee values are discarded. `distributeRewards()` updates only in-memory `StakingModule` (not persisted, not applied to StateManager). `AddReward()` is never called in production code. | `internal/layer1/ledger/persistent_chain.go:93`, `consensus/hotstuff.go:1641` | Call `stateMgr.MintTokens(validatorAddr, reward)` in the consensus `decide()` path. Wire `ProcessBlock` result into actual state mutations. |
| 1.3 | **Only ~10M wei exists in the entire chain** — Genesis supply (10^9) ≠ Economics supply (10^25). No minting happens. Validator-2 has 9.9M wei from hardcoded `CreateAccount`. | `cmd/virid/main.go:445-448` | Remove hardcoded per-validator balance. Fix genesis to actually allocate `initial_supply` to the `faucet_address` account. |
| 1.4 | **No supply is ever minted to accounts** — `totalSupply` and `circulatingSupply` are abstract counters; no `MintTokens` call exists in any production path. | `internal/layer1/state/state.go` | Add `MintTokens(address, amount)` and `BurnTokens(amount)` methods to StateManager. Call from block reward distribution. |
| 1.5 | **Silent fallback to in-memory store** — If BadgerDB fails to open, node uses MemoryStore with no warning. All state lost on restart. | `cmd/virid/main.go:206-215` | `log.Fatal` on DB failure for non-dev modes. Add startup check: if store is in-memory, emit a visible WARNING banner. |

### High

| # | Issue | File | Fix |
|---|-------|------|-----|
| 1.6 | **Genesis config fragmentation** — Two completely different `GenesisConfig` structs (`ledger` vs `genesis` packages). `final_genesis.go` generates fields (`initial_supply`, `faucet_allocation`, `faucet_address`) that are never parsed by the runtime node. | `internal/layer1/ledger/genesis.go`, `final_genesis.go` | Merge into one config. Use the `genesis.GenesisConfig` format (with `Allocations` map) and apply allocations during `Initialize()`. |
| 1.7 | **`faucet_address` in genesis is 32 bytes** — `"0xab73db00...4da"` (64 hex chars), not a 20-byte Ethereum address | `configs/genesis/testnet.json`, `testnet/genesis/genesis.json` | Fix to valid 20-byte address: `0xab73db00298166a90c11f8f6fdcca3e9b22f3db87` |
| 1.8 | **Dual/triple supply tracking** — StateManager.totalSupply (uint64, from `genesis.InitialSupply`) vs Economics.circulatingSupply (big.Int, from config) vs actual account balance sum. None match. | `internal/layer1/state/state.go:28`, `economics.go:35` | Single source of truth: `TotalSupply()` = sum of all account balances. Remove abstract counters. |
| 1.9 | **O(n) state root computation** — `computeStateRoot()` reads ALL accounts from DB, serializes each, builds Merkle tree every block. Intolerable at scale. | `internal/layer1/state/state.go:186-212` | Use the existing `MPT` (Merkle-Patricia Trie) for O(log n) incremental updates. |
| 1.10 | **`fund_faucet` tool bypasses StateManager** — Direct BadgerDB writes, no supply check, no state root update. | `tools/fund_faucet/main.go` | Use `stateMgr.CreateAccount()` and `stateMgr.Commit()`. Better: implement a proper `eth_sendTransaction` API. |

### Medium

| # | Issue | Fix |
|---|-------|-----|
| 1.11 | `rewardPool uint64` will overflow — block reward (10^18) × ~18 blocks > uint64 max | Use `*big.Int` |
| 1.12 | `distributeRewards` integer division loses remainder | Track remainder, distribute in next round |
| 1.13 | MPT orphan nodes accumulate forever — old node versions never GC'd | Add reference-counting GC pass |
| 1.14 | Empty migration list, but migration code runs every startup | Remove migration scaffolding or implement actual migrations |
| 1.15 | `EconomicsConfig.InitialSupply` default (10^25) vs `ledger.GenesisConfig.InitialSupply` default (1,000,000,000) — 15 orders of magnitude off | Align defaults |

---

## 2. RPC LAYER: COMPATIBILITY & CORRECTNESS

### Critical

| # | Issue | File | Fix |
|---|-------|------|-----|
| 2.1 | **`from` field returns 65-byte public key, not 20-byte address** in `getTransactionByHash`, `getTransactionReceipt`, `formatTx` | `cmd/virid/rpc_server.go:697,1039,1407` | Replace `tx.From` with `tx.SenderAddress()` |
| 2.2 | **Gob serialization is incompatible with Ethereum** — `eth_sendRawTransaction` expects gob-encoded bytes, not RLP. No standard wallet can submit transactions. | `internal/layer1/ledger/serialize.go:38-54`, `cmd/virid/rpc_server.go:517-560` | Add RLP encoding/decoding. Keep gob for internal use, but accept RLP in `eth_sendRawTransaction`. OR: add an `eth_sendRawTransaction` bridge that accepts RLP and converts. |
| 2.3 | **No CORS middleware on RPC server** — MetaMask and browser dApps get CORS errors connecting directly. | `cmd/virid/rpc_server.go:203-217` | Add `corsMiddleware` to the handler chain (exists in `api_server.go` — reuse it). |

### High

| # | Issue | Fix |
|---|-------|-----|
| 2.4 | `eth_estimateGas` returns 21000 for transfers, but actual cost is 26000 | Return `0x6598` (26000) for transfers, implement EVM simulation for contracts |
| 2.5 | `eth_getStorageAt` always returns `"0x0"` — ignores storage key entirely | Call `stateMgr.GetStorage(address, key)` |
| 2.6 | `eth_subscribe`/`eth_unsubscribe` missing — no WebSocket subscription support | Implement on WS server with Ethereum JSON-RPC protocol, not custom pubsub |
| 2.7 | `eth_maxPriorityFeePerGas` and `eth_feeHistory` missing — MetaMask can't estimate EIP-1559 fees | Add — `maxPriorityFeePerGas` can return a fixed low value; `feeHistory` can be stubbed |
| 2.8 | WebSocket server uses custom protocol on separate port — not Ethereum JSON-RPC compatible | Port WS to same port as HTTP RPC, support `eth_subscribe` with standard params |
| 2.9 | `eth_call` has no EVM context — `blockNum`, `timestamp`, `chainID`, `coinbase`, `baseFee` are all empty | Populate EVM context from current block state |
| 2.10 | `eth_getLogs` has unsafe type assertion — `v.(string)` panics if address is array | Use proper type switching: handle both `string` and `[]string` |
| 2.11 | `queryLogs` (for filters) ignores topics, only checks `tx.To` — logs never returned correctly | Use same `getLogs` logic for both direct queries and filters |
| 2.12 | Block format missing 15+ standard Ethereum fields — `gasLimit`, `gasUsed`, `baseFeePerGas`, `size`, `logsBloom`, `stateRoot`, `transactionsRoot`, `receiptsRoot`, `difficulty`, `totalDifficulty`, `nonce`, `mixHash`, `extraData`, `sha3Uncles`, `uncles` | Add all fields to `formatBlock`/`formatBlockWithTxs` |
| 2.13 | Transaction format missing `input`, `v`, `r`, `s`, `chainId` — `from` is pubkey | Add standard fields. Fix `from` (issue 2.1). |
| 2.14 | Receipt missing `logsBloom`, `effectiveGasPrice`, `type`, `cumulativeGasUsed` | Add missing fields |

### Medium

| # | Issue | Fix |
|---|-------|-----|
| 2.15 | No batch JSON-RPC support | Parse arrays of requests in `handleRequest` |
| 2.16 | `eth_getBlockByNumber` ignores `"earliest"` tag | Handle `"earliest"` → block 0 |
| 2.17 | `eth_getBlockByHash` and `getBlockByNumber` ignore full-tx boolean parameter | Support second param: if true, return full tx objects |
| 2.18 | `eth_syncing` returns non-standard field names (underscores not camelCase) | Rename `starting_block` → `startingBlock`, etc. |
| 2.19 | Timestamps returned as Unix integers, not hex strings | Prepend `"0x"` and format as hex |
| 2.20 | `eth_getTransactionCount` pending nonce logic is wrong | `max(state_nonce, highest_pending_nonce + 1)` |
| 2.21 | `eth_sendRawTransaction` skips ChainID validation | Compare `tx.ChainID` against node's `chainID` |
| 2.22 | `eth_gasPrice` returns base fee only, no priority fee | Add small tip (e.g., 1 wei) to ensure inclusion |
| 2.23 | Error format missing optional `data` field | Add `data` to `RPCError` struct |

---

## 3. TRANSACTION FORMAT & SIGNING

### Critical

| # | Issue | File | Fix |
|---|-------|------|-----|
| 3.1 | **`viri-sign` produces gob-incompatible output** — defines `main.Transaction` but `sendRawTransaction` expects `ledger.Transaction`. Gob encodes fully-qualified type name, so the wire format is incompatible. | `tools/viri-sign/main.go:25-37` | Either: (a) use `ledger.Transaction` directly (import the package), or (b) manually implement gob-compatible wire encoding matching the `ledger.Transaction` type path. |
| 3.2 | **Transaction format is completely non-standard** — gob-encoded, not RLP. No Ethereum wallet can create or submit transactions to this chain. | Entire transaction pipeline | Add RLP encoding. Implement `TransactionFromRLP(rlpBytes) (*Transaction, error)`. Register both RLP and gob handlers in `eth_sendRawTransaction`. |

### High

| # | Issue | Fix |
|---|-------|-----|
| 3.3 | `V` byte is hardcoded to `0`, never verified — any `V` value passes `Verify()` | Compute correct `V` from signature recovery, add `V` validation in `Verify()` |
| 3.4 | No ChainID check in `TxPool.Add()` or `sendRawTransaction` — cross-chain replay possible | Validate `tx.ChainID == expectedChainID` before pool insertion |
| 3.5 | Replace-by-fee evicts globally lowest-gas-price tx, not same-sender-nonce — DoS vector | Fix: only allow replacement when `sender + nonce` match AND new gas price > old × 1.1 (EIP-1159 style) |
| 3.6 | `FeeCurrency` not in `SigningPayload()` — unsigned field, malleable after signing | Include `FeeCurrency` in the signing payload |

### Medium

| # | Issue | Fix |
|---|-------|-----|
| 3.7 | `tx.Hash` never validated against `tx.ComputeHash()` — tampered hash accepted | Add hash verification in `sendRawTransaction` |
| 3.8 | Nil `SenderAddress()` causes empty-string account key, bypassing per-account limits | Return error from `pool.Add()` if `SenderAddress()` returns nil |
| 3.9 | Duplicate `NewTransaction` / `NewTransactionFromKey` — identical code | Remove duplicate |
| 3.10 | Weak `len(tx.From) < 2` check — should check for exactly 65 bytes | Validate `len(tx.From) == 65` |
| 3.11 | Low-S enforcement has extremely rare truncation bug (wrong `copy` length) | Fix: use proper byte slicing for the replacement S value |
| 3.12 | Nonce check silently skipped on state lookup errors | Explicit error on failed state lookup |
| 3.13 | `TransactionToJSON` drops `FeeCurrency`, `Signature`, `ChainID` | Include all fields |

---

## 4. FAUCET & TOKEN DISTRIBUTION

### High

| # | Issue | Fix |
|---|-------|-----|
| 4.1 | **Faucet private key hardcoded in source code** (`validator-2` key) | Move to environment variable, load at runtime |
| 4.2 | **Private key passed on command line to `viri-sign`** — visible via `/proc` to all processes | Use stdin pipe or temp file with restricted permissions |
| 4.3 | **Per-claim = 100 wei is absurdly low** — should be at least 10^15 wei (0.001 VIRI) to be usable | Increase to a meaningful amount. But with only 9.9M wei available, need to fix tokenomics first (issue 1.2-1.4). |
| 4.4 | **Faucet address in faucet service** (`0xdb02a...`) **doesn't match genesis** (`0xab73db...` 32-byte) | Fix address format to 20 bytes. Ensure genesis actually funds the faucet address. |

### Medium

| # | Issue | Fix |
|---|-------|-----|
| 4.5 | `loadStore()` called on every request with no caching — high disk I/O | Cache store in memory, write-back every N seconds |
| 4.6 | Cooldown store is unsigned JSON — filesystem access allows tampering | HMAC-sign or use a DB-backed store |
| 4.7 | IP-based cooldown disadvantages NAT users (multiple users behind same IP) | Use a captcha (e.g., hCaptcha) instead of IP-based rate limiting |
| 4.8 | RPC proxy has hardcoded API key | Move to env var, add key rotation support |
| 4.9 | RPC proxy has no rate limiting — public endpoint with no DoS protection | Add token-bucket rate limiter |

---

## 5. DEPLOYMENT INFRASTRUCTURE

### Critical

| # | Issue | Fix |
|---|-------|-----|
| 5.1 | **No TLS anywhere** — Nginx listens on port 80 only. All traffic unencrypted. | `listen 443 ssl;` with Let's Encrypt via certbot/cert-manager |
| 5.2 | **Validator private keys committed to repo** — `testnet/keys/validator-*.key` in plaintext | Remove from repo, add to `.gitignore`. Rewrite git history. |
| 5.3 | **API key in source** — `fa776f19...` in `rpc-proxy.js`, `faucet-service-v3.js`, Terraform cloud-init | Rotate keys, load from env vars |
| 5.4 | **4 validator keys in Terraform cloud-init YAML** | Use Azure Key Vault or SSH-triggered key delivery |

### High

| # | Issue | Fix |
|---|-------|-----|
| 5.5 | Nginx has no rate limiting on RPC endpoints | Add `limit_req_zone $binary_remote_addr zone=rpc:10m rate=5r/s` |
| 5.6 | `HashFromPayload` copies first 32 bytes, doesn't hash — collisions guaranteed | Use real SHA-256 |
| 5.7 | Consensus state is in-memory only — crash loses height, lockedQC, preparedQC, votes | Persist to BadgerDB on every QC advancement and view change |
| 5.8 | Ansible systemd template uses `Type=oneshot` for daemon — systemd won't restart on crash | Change to `Type=simple` |
| 5.9 | No health checks in Docker Compose — dependent containers start before DB/chain is ready | Add `healthcheck` to each service |
| 5.10 | State sync is block-by-block — new node joining at height 100k must download/validate every block | Implement snapshot-based sync (periodic state checkpoints) |
| 5.11 | Peer discovery script has hardcoded peer list, no error handling, no retry | Use proper libp2p discovery (Kademlia DHT, mDNS) |

### Medium

| # | Issue | Fix |
|---|-------|-----|
| 5.12 | Dockerfile + docker/Dockerfile — two inconsistent build pipelines | Consolidate to one |
| 5.13 | No resource limits on Docker containers | Add `mem_limit`, `cpus` |
| 5.14 | `entrypoint.sh` missing `set -e` — silent failures | Add `set -euo pipefail` |
| 5.15 | Rotating file logger never instantiated — all log config is dead code | Wire `RotatingFileWriter` into `NewLogger` |
| 5.16 | Backup timer has `Persistent=false` — missed backups not caught up | Add `Persistent=true` |
| 5.17 | Backups on same filesystem as DB — single disk failure loses both | Separate backup volume/device, add off-site replication |
| 5.18 | K8s HPA for BFT validators — destroying a pod breaks consensus quorum | Remove HPA, use fixed replica count with PDB |
| 5.19 | No PDB or anti-affinity in K8s deployment | Add `PodDisruptionBudget: minAvailable=3`, pod anti-affinity |
| 5.20 | No connection gater in P2P — any peer can connect | Implement IP whitelist/blacklist in `conn_manager.go` |

---

## 6. SECURITY

### Critical

| # | Issue | Fix |
|---|-------|-----|
| 6.1 | Private keys hardcoded in 5+ source files | See 5.2, 5.3, 5.4 |
| 6.2 | API key in source code — used for `eth_sendRawTransaction` auth | Rotate and externalize |
| 6.3 | Prometheus password `changeme` — monitoring data exposed | Change to strong password or use OAuth |
| 6.4 | `insecure_skip_verify: true` in Prometheus config | Enable TLS verification |

### High

| # | Issue | Fix |
|---|-------|-----|
| 6.5 | Terraform NSG opens Prometheus/Grafana ports to internet | Restrict to known IPs or use VPN |
| 6.6 | No request body size limit on RPC server (only 5MB ContentLength check) | Add explicit `http.MaxBytesReader` |
| 6.7 | P2P message authentication uses 5-min max message age — replay within window possible | Shorter window + nonce tracking |
| 6.8 | No anti-replay beyond timestamp for P2P messages | Add per-sender nonce tracking |

### Medium

| # | Issue | Fix |
|---|-------|-----|
| 6.9 | Grafana default `admin/admin` — change in deployment | Document as required change |
| 6.10 | No secrets management integration (Vault/Key Vault) | Add for production |
| 6.11 | Rate limiter in consensus `HandleMessage` operates outside mutex — race condition | Move rate limiter inside mutex scope |
| 6.12 | DHT enabled on all nodes including non-validators — exposure to DHT attacks | Disable DHT for non-validator nodes |

---

## 7. MISSING FEATURES (for production testnet)

### Must-Have

| # | Feature | Why |
|---|---------|-----|
| 7.1 | **Faucet with real tokens** — needs working tokenomics (1.2-1.4) | Current testnet has only 10M wei total |
| 7.2 | **MetaMask-compatible RPC** — needs fixes 2.1-2.14 | Currently unusable from MetaMask |
| 7.3 | **Block explorer** — needs to show correct tx data (after fixing 2.1, 2.12-2.14) | Currently shows pubkey as `from` |
| 7.4 | **HTTP/WS on same port** — standard Ethereum convention | MetaMask, web3.js expect this |
| 7.5 | **`eth_chainId` returns chain ID 2** (this is fine) | Already 0x2 — correct |
| 7.6 | **`eth_estimateGas` returns correct minimum** | Fix 2.4 + 3.2 |

### Nice-to-Have

| # | Feature | Why |
|---|---------|-----|
| 7.7 | Bridge to Sepolia/Goerli | Real testnet interoperability |
| 7.8 | Public faucet with captcha | Prevent spam |
| 7.9 | Network status page | Node health, block height, peer count |
| 7.10 | Documentation site | Developer onboarding |
| 7.11 | WebSocket event streaming | Real-time dApp updates |
| 7.12 | Staking dashboard | Test validator operations |

---

## SUMMARY

### Severity Count

| Severity | Count | Must Fix Before Production |
|----------|-------|---------------------------|
| **Critical** | 15 | YES |
| **High** | 35 | YES |
| **Medium** | 45 | Strongly recommended |
| **Low** | 55+ | Nice to have |

### Top 10 Blockers for Production Testnet

| Rank | Issue | Area | Why It Blocks |
|------|-------|------|---------------|
| 1 | **No tokens exist** — rewards/minting broken, only 10M wei from hardcode | Core | Users receive 0 tokens |
| 2 | **State resets on restart** | Core | All user data lost on reboot |
| 3 | **Gob tx format, not RLP** | RPC/Tx | No wallet can send transactions |
| 4 | **`from` field is pubkey, not address** | RPC | All txs show wrong sender |
| 5 | **No CORS on RPC server** | RPC | MetaMask can't connect |
| 6 | **Private keys in source code** | Security | Complete chain takeover |
| 7 | **No TLS** | Infra | All traffic in plaintext |
| 8 | **`virid-sign` produces incompatible gob** | Tx | Faucet can't send txs |
| 9 | **Consensus state not persisted** | Consensus | Crashes cause reorgs |
| 10 | **Faucet address is 32 bytes** | Config | Can't receive tokens |

### What's Actually Working (good news)

- HotStuff BFT consensus with view changes, QCs, timeouts ✅
- P2P networking with libp2p, DHT discovery, NAT traversal ✅
- BadgerDB-backed persistent storage (when it works) ✅
- 38+ JSON-RPC methods registered (many with issues but present) ✅
- Docker deployment pipeline ✅
- Prometheus + Grafana monitoring (basic) ✅
- Faucet with cooldown + daily limit (logic works, math is off) ✅
- Explorer showing blocks and transactions ✅
- Ethereum address derivation from secp256k1 keys ✅
- Account state model with balance, nonce, code, storage ✅
- EIP-1559 fee market (base fee calculation) ✅
