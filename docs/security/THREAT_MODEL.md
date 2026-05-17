# Viri Blockchain — Formal Threat Model

**Document Version:** 0.1.0
**Date:** 2026-05-17
**Methodology:** STRIDE per component + DREAD for risk scoring

---

## 1. System Overview

Viri is a 3-layer modular blockchain: L1 consensus + L2 execution engine + L3 application layer. All three layers run in a single Go binary with no external dependencies.

### Trust Boundary Diagram

```
User / Client Apps
    │
    ├──► L3 API (REST) ──► L3 Governance / Bridge / Interop / Intent
    │                            │
    │                            ▼
    ├──► L2 RPC (JSON-RPC) ──► L2 EVM / WASM / ZK / Gas / MEV / Rollups
    │                            │
    │                            ▼
    └──► L1 P2P ──► L1 Consensus (HotStuff-2 BFT) ──► L1 State (MPT)
                              │
                              ▼
                         BadgerDB
```

### Trust Assumptions

| Trust Anchor | Rationale |
|-------------|-----------|
| Honest validator supermajority (>2/3 stake) | BFT consensus requires this |
| Node operators follow security hygiene | Key management, updates, monitoring |
| Infrastructure providers (cloud, colo) | Hardware/network not tampered |
| No L1 consensus break | HotStuff-2 proven safe under N>3F |

---

## 2. Threat Model by Layer

### 2.1 L1 Consensus (HotStuff-2 BFT)

| # | Threat | STRIDE | Attack Vector | Impact | Risk | Mitigation |
|---|--------|--------|---------------|--------|------|------------|
| T1 | Malicious leader proposes invalid block | **S**poofing | Byzantine leader sends block with invalid state transition | Chain halt or fork | **Critical** | QC verification: replicas validate prepareQC before accepting; TLA+ verified invariant |
| T2 | Equivocation — validator votes for two different blocks at same height | **S**poofing | Byzantine validator sends contradicting votes | Fork | **Critical** | Double-sign detection + slashing per `internal/layer1/slashing`; TLA+ verified NoDoubleCommit |
| T3 | Phantom QC — validator accepts fake quorum certificate | **T**ampering | Byzantine validator crafts QC with invalid signatures | Replica locks bad value | **High** | QC signature verification per `internal/layer1/consensus`; QuorumIntersection TLA+ invariant |
| T4 | Replay of old messages | **R**epudiation | Attacker replays captured messages from previous views | Confusion, wasted CPU | **Medium** | View-based nonce + height sequencing; duplicate message detection |
| T5 | Denial of service — message flood | **D**enial of Service | Attacker floods P2P with garbage messages | Node falls behind | **High** | Rate limiter + peer reputation scoring per `internal/layer1/p2p` and `internal/pkg/security` |
| T6 | Eclipse attack — peer isolation | **D**enial of Service | Attacker controls all outbound connections | Node partitioned from honest chain | **High** | Max peers = 100, DHT discovery, peer diversity scoring |
| T7 | Timing attack on leader election | **I**nformation Disclosure | Attacker uses message timing to predict next leader | MEV opportunity | **Low** | Round-robin proposer selection per view |
| T8 | Fee market manipulation | **E**levation of Privilege | Attacker floods with high-gas txs to skew base fee | Network congestion | **Medium** | EIP-1559 base fee adjustment per block; block gas limit enforced |

### 2.2 L1 P2P Networking

| # | Threat | STRIDE | Attack Vector | Impact | Risk | Mitigation |
|---|--------|--------|---------------|--------|------|------------|
| T9 | Sybil attack — fake peer identities | **S**poofing | Spawn many low-cost peers to dominate peer table | Eclipse, censorship | **High** | Peer reputation scoring, ban threshold, connection limits |
| T10 | Man-in-the-middle on P2P messages | **T**ampering | Intercept and modify blocks/votes in transit | Invalid state committed | **Critical** | Message authentication with ECDSA P-256 signatures per message |
| T11 | Peer table poisoning | **T**ampering | Inject bad peer records into DHT | Node connects to malicious peers | **Medium** | Peer validation on connect; reputation scoring for persistent offenders |
| T12 | Gossip flood amplification | **D**enial of Service | Inject messages that trigger rebroadcast to all peers | Network saturation | **High** | Duplicate message cache (per-peer), rate limiting, message size limits |

### 2.2 L2 Execution (EVM + WASM)

| # | Threat | STRIDE | Attack Vector | Impact | Risk | Mitigation |
|---|--------|--------|---------------|--------|------|------------|
| T13 | EVM opcode gas underprice | **T**ampering | Deploy contract that exploits underpriced opcode to exhaust block gas | Block stall, DoS | **High** | Gas metering per opcode; fuzz-verified (`FuzzEVMExecution`) |
| T14 | EVM reentrancy | **E**levation of Privilege | Malicious contract re-enters caller before state update | Theft of funds | **High** | Checks-effects-interactions pattern recommended; no protocol-level reentrancy guard (EVM-standard) |
| T15 | ZK proof forgery | **T**ampering | Submit forged Groth16 proof with fake nullifier | Spend notes not owned | **Critical** | On-chain gnark verifier precompile; `TestPrecompileGnarkVerifyTampered` |
| T16 | WASM runtime escape | **E**levation of Privilege | WASM contract accesses host memory outside sandbox | Node compromise | **Critical** | WASM runtime sandboxed with gas metering; limited host imports |
| T17 | Gas estimation oracle manipulation | **I**nformation Disclosure | Craft transactions that produce misleading gas estimates | Underpriced txs get stuck | **Low** | Gas estimator uses historical blocks, not immediate mempool |
| T18 | MEV — front-running via tx ordering | **E**levation of Privilege | Validator reorders txs for profit | User value extraction | **High** | Fair-ordering + batch sequencing (`internal/layer2/mev`); TEE-based sequencing planned |

### 2.3 L3 Application Layer

| # | Threat | STRIDE | Attack Vector | Impact | Risk | Mitigation |
|---|--------|--------|---------------|--------|------|------------|
| T19 | Governance proposal spam | **D**enial of Service | Submit many low-quality proposals | Voting fatigue, chain bloat | **Low** | Proposal stake requirement enforced |
| T20 | Governance vote buying | **E**levation of Privilege | Bribe validators to vote maliciously | Protocol takeover | **Critical** | Token-weighted voting + veto mechanism; off-chain social consensus as final backstop |
| T21 | Bridge — invalid transfer attestation | **T**ampering | Fake multi-sig signatures to release locked assets | Theft of bridged assets | **Critical** | Threshold signature verification (2/3+); replay protection; per validator signature validation |
| T22 | Bridge — replay attack across chains | **R**epudiation | Replay same transfer on destination chain | Double withdrawal | **High** | Nonce-based replay protection per `internal/layer3/bridge`; tested in `TestPrivacyBridgeDoubleSpendPrevention` |
| T23 | Intent solver fraud | **S**poofing | Solver submits invalid fill for intent | User receives wrong asset | **High** | Intent fill verification with on-chain checks; invalid fill rejection per `internal/layer3/intent` |
| T24 | API key brute force | **T**ampering | Guess API keys via enumeration | Unauthorized API access | **Medium** | Constant-time comparison (`internal/pkg/security`); rate limiting per IP |
| T25 | API rate limit bypass | **D**enial of Service | Rotate IPs or keys to exceed limits | Resource exhaustion | **Medium** | Token bucket + per-method rate limiting; connection draining |

### 2.4 Infrastructure

| # | Threat | STRIDE | Attack Vector | Impact | Risk | Mitigation |
|---|--------|--------|---------------|--------|------|------------|
| T26 | Key compromise — validator private key | **I**nformation Disclosure | Attacker reads encrypted keystore file | Slashing, equivocation | **Critical** | AES-256-GCM encrypted keystore with scrypt key derivation; passphrase required; HSM support documented |
| T27 | Disk failure | **D**enial of Service | Hardware failure on validator node | State loss | **High** | BadgerDB with WAL; backup procedures in DISASTER_RECOVERY.md; snap sync for fast recovery |
| T28 | DDoS on RPC endpoint | **D**enial of Service | Saturate RPC with requests | Users cannot interact | **High** | Rate limiter + connection limiter + DDoS detector with per-IP backpressure (`internal/pkg/security`) |
| T29 | DNS spoofing for bootstrap peers | **T**ampering | DNS cache poisoning points to attacker node | Connect to malicious peer | **High** | Peer ID verification on connect; bootstrap IPs documented for manual verification |

---

## 3. Risk Summary

| Risk Level | Count | Critical Threats |
|------------|-------|------------------|
| **Critical** | 5 | T1 (proposal), T2 (equivocation), T6 (eclipse), T15 (ZK forgery), T16 (WASM escape), T21 (bridge fraud) |
| **High** | 10 | T3, T5, T9, T10, T12, T13, T14, T18, T22, T23, T26, T27, T28, T29 |
| **Medium** | 4 | T4, T8, T17, T24, T25 |
| **Low** | 1 | T7 |

### Mitigation Coverage

| Mitigation | Threats Covered |
|------------|-----------------|
| TLA+ formal verification | T1, T2, T3 |
| ECDSA P-256 signatures | T10, T21, T26 |
| Fuzz testing (17 harnesses) | T13, T14 |
| Rate limiting + DDoS protection | T5, T12, T24, T25, T28 |
| Peer reputation scoring | T5, T9, T11 |
| Slashing for double-sign | T2 |
| Encrypted keystore | T26 |
| Disaster recovery runbook | T27 |
| Multi-sig threshold | T21, T22 |

---

## 4. Attack Trees

### 4.1 Chain Fork via Byzantine Validator

```
Goal: Cause two honest replicas to commit different blocks at same height
├── Tactic: Equivocation (T2) — signed by double-sign slashing
│   └── Countermeasure: Slashing logic detects duplicate votes
├── Tactic: Phantom QC (T3) — requires forging QC signatures
│   └── Countermeasure: ECDSA verification on every QC; TLA+ invariant
└── Tactic: Network delay tricks validator into accepting stale QC
    └── Countermeasure: View-based sequencing; timeout certificates
```

### 4.2 Bridge Asset Theft

```
Goal: Release bridged assets without legitimate lock on source chain
├── Tactic: Forge validator signatures (T21)
│   ├── Requires compromising >1/3 of validator keys
│   └── Countermeasure: Threshold 2/3+; slashing
├── Tactic: Replay attack (T22)
│   └── Countermeasure: Nonce + source chain ID in transfer payload
└── Tactic: Eclipse validator to sign without observing lock
    └── Countermeasure: Multiple bootstrap peers; DHT discovery
```

---

## 5. Security Controls Mapping

| Control | Location | Coverage |
|---------|----------|----------|
| Authentication | `internal/pkg/security/api_key_auth.go` | API keys, Bearer tokens |
| Authorization | `internal/layer3/governance` | Stake-weighted voting |
| Cryptography | `internal/layer1/crypto` | ECDSA P-256, AES-256-GCM, SHA-256, Keccak256 |
| Rate limiting | `internal/pkg/security/rate_limit.go` | Token bucket, per-method |
| DDoS protection | `internal/pkg/security/dos_protector.go` | Slow query, connection drain |
| Input validation | `internal/layer2/vm` | Gas metering, sandboxing |
| Audit logging | `internal/pkg/audit` | Security events, operation log |
| Monitoring | `monitoring/prometheus.yml`, `monitoring/alerting_rules.yml` | Node health, consensus, network |

---

## 6. Residual Risk

| Risk | Why Accepted | Compensating Control |
|------|-------------|---------------------|
| Governance attack via whale accumulation | Cost to acquire >1/3 stake exceeds expected gain | Social consensus veto; fork as last resort |
| Quantum computer breaks ECDSA | Cryptographically relevant quantum computer does not exist yet | Pluggable signature schemes; ML-DSA and SPHINCS+ implemented |
| 0-day in Go runtime or dependencies | No practical mitigation for language-level vulnerabilities | Dependency scanning in CI; minimal dependency surface |

---

## 7. Review Cadence

| Review Type | Frequency | Owner |
|-------------|-----------|-------|
| Threat model review | Quarterly | Security team |
| Dependency audit | Monthly | CI pipeline |
| Key rotation | Annual | Validator operators |
| Penetration test | Pre-mainnet + annually | External firm |
| Bug bounty | Post-mainnet | Community via SECURITY.md |
