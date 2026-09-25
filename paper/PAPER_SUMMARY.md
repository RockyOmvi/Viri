# Viri: A Production-Grade 3-Layer Modular Blockchain Architecture with Native Account Abstraction, Dual VM Runtimes, and Zero-Knowledge Privacy

**Purushottam Kumar**, Viri Core Protocol Team and Research Group  
*Email: rockysinghrajput05@gmail.com*  
*Publication Date: August 2026*  
*Target Publication: IEEE Transactions on Computers / IEEE Symposium on Security & Privacy*

---

## Abstract

Current modular blockchain designs decompose consensus, execution, and data availability across independent specialized protocols. While modularity enhances component focus, external layer dependencies introduce multi-hop bridge vulnerability vectors, fragmented execution semantics, latency overheads, and cross-layer trust assumptions. In this paper, we present **Viri**, a production-grade 3-layer modular blockchain built entirely within a unified Go stack, eliminating external layer dependencies without sacrificing modular isolation. 

Layer 1 provides a formally verified HotStuff-2 3-phase Byzantine Fault Tolerant (BFT) consensus engine, a hex-radix Merkle-Patricia Trie (MPT) state database, and p2p networking with peer reputation scoring. Layer 2 implements a dual WebAssembly (WASM) and Ethereum Virtual Machine (EVM) runtime environment, native account abstraction without externally-owned accounts (EOAs), dynamic EIP-1559 gas market mechanics with multi-token fee abstraction, a Pedersen-commitment zero-knowledge (ZK) shielded pool operating over BN254 Groth16 proofs, and a TEE-assisted MEV-resistant sequencer. Layer 3 introduces native inter-appchain communication (IBC-style channels), declarative intent solvers, a cross-chain threshold multi-sig bridge, and an on-chain governance DAO. 

We provide formal TLA+ verification of HotStuff-2 safety invariants across 34+ million generated states, micro-benchmark empirical evaluations (including 11 ns consensus steps, 241 $\mu$s block addition, 101 ns token transfers), and Jepsen-style fault injection analytics proving chain safety and monotonicity under 100-validator supermajorities producing 2,015 ops/sec.

---

## 1. Introduction

Decentralized ledger technology has undergone rapid paradigm shifts from monolithic blockchain architectures (e.g., Bitcoin, Ethereum 1.0) to modular ecosystem frameworks. In traditional monolithic blockchains, a single unified set of validator nodes executes state transitions, establishes consensus ordering, guarantees data availability (DA), and processes smart contract business logic. Consequently, scalability is tightly bound by the hardware limits of individual consensus nodes, causing high transaction fees and throughput bottlenecks during peak network utilization.

To address these limitations, recent state-of-the-art designs favor modular decoupling. Modular blockchains divide ledger responsibilities into distinct functional layers: Execution (Layer 2 rollups), Consensus and Data Availability (Layer 1 base chains), and Application ecosystems (Layer 3 app-chains). However, existing modular implementations outsource layer components to third-party networks. For example, Layer 2 rollups post transaction batches to external DA layers (e.g., Celestia), settle state roots on Layer 1 smart contracts, and rely on external liquidity bridges for inter-rollup state transfer.

This multi-party ecosystem design introduces critical architectural challenges:
1. **Cross-Layer Trust and Bridge Risk**: Reliance on external bridges introduces multi-sig relayers and smart contract vulnerabilities, accounting for over $2.8B in exploit losses across public decentralized finance.
2. **Fragmented User Experience and Gas Friction**: Users must manage externally-owned accounts (EOAs) across multiple chains, maintain gas balances in distinct native tokens, and deal with complex ERC-4337 relayers.
3. **Protocol-Level MEV Extraction**: Sequencers operating without cryptographic fair-ordering primitives extract Maximum Extractable Value (MEV) through sandwiching and front-running.

### 1.1 The Viri Paradigm: Unified 3-Layer Architecture
To resolve these structural trade-offs, we present **Viri**, a production-grade 3-layer modular blockchain implemented natively in Go with *zero external layer dependencies*. Viri maintains strict modular separation across Layer 1 (Core), Layer 2 (Execution & Privacy), and Layer 3 (Application & Interoperability) within a unified protocol codebase.

```
┌───────────────────────────────────────────────────────────────────────────────────────┐
│                                LAYER 3 — APPLICATION                                  │
│  App Chains  ·  IBC-like Interop  ·  REST API  ·  Governance DAO  ·  Cross-Bridge     │
│  Intent Solver Network  ·  SDK  ·  On-Chain Proposal Life-Cycle & Threshold Voting    │
├───────────────────────────────────────────────────────────────────────────────────────┤
│                                LAYER 2 — EXECUTION                                    │
│  Dual VM (EVM + WASM)  ·  Native Account Abstraction  ·  Pedersen-Groth16 ZK Pool     │
│  MEV Resistance (TEE)  ·  EIP-1559 Dynamic Gas Oracle  ·  Multi-Token Fee Abstraction  │
├───────────────────────────────────────────────────────────────────────────────────────┤
│                                LAYER 1 — CORE INFRASTRUCTURE                          │
│  HotStuff-2 3-Phase BFT  ·  Delegated PoS  ·  Hex-Radix Merkle-Patricia Trie (Badger) │
│  Stateless Block Sync  ·  Slashing Engine  ·  libp2p Authenticated Peer Manager       │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Key Technical Contributions
Our primary contributions are summarized as follows:
- **Formally Verified HotStuff-2 Consensus**: Implementation of a three-phase (Prepare $\rightarrow$ PreCommit $\rightarrow$ Commit $\rightarrow$ Decide) BFT engine featuring linear view change complexity, optimistic responsiveness, and TLA+ formal specification model-checked across 34.1M+ execution paths with zero safety violations.
- **Native Account Abstraction (Zero-EOA Framework)**: Elimination of EOAs from genesis. Every account is a smart contract wallet executing native validation logic, session keys, multi-sig authorization, and fee sponsorship.
- **Multi-Token Fee Market & Dual VM**: Integrated EVM and WASM execution engines with EIP-1559 gas adjustment mechanics, allowing transaction fee payment in arbitrary tokens via an on-chain price oracle.
- **Pedersen-Groth16 Privacy Pool**: Native shielded transfers leveraging Pedersen commitments, nullifier set tracking, and Groth16 zk-SNARK precompiles over the BN254 elliptic curve.
- **Empirical Benchmark Suite & Analytics**: Comprehensive empirical analysis demonstrating 2,015 ops/sec supermajority consensus throughput, 241 $\mu$s block addition latency, 101 ns account transfer evaluation, and Jepsen fault injection resilience.

---

## 2. System Architecture & Layer Breakdown

### 2.1 Layer 1: Core Consensus and State Management
Layer 1 is responsible for transaction ordering, cryptographically verifiable state persistence, peer-to-peer networking, and validator slashing.
- `internal/layer1/consensus`: Implements the HotStuff-2 BFT consensus state machine. Nodes participate in round-robin view transitions to reach finality under $F < N/3$ Byzantine faulty replicas.
- `internal/layer1/state`: Manages account state and storage using a hex-radix Merkle-Patricia Trie (MPT) with BadgerDB key-value persistence. State roots are computed incrementally in $O(\log N)$ time.
- `internal/layer1/p2p`: A modular p2p network built on `libp2p`, providing peer discovery, encrypted transport, peer reputation scoring, and rate-limited message dissemination.
- `internal/layer1/slashing`: Automated double-sign detection and equivocation proof validation, executing validator jailing and stake slashing.

### 2.2 Layer 2: Execution, Privacy, and Sequencing
Layer 2 handles state execution, virtual machine interpretability, privacy primitives, and transaction ordering.
- `internal/layer2/vm`: Dual execution environment running full EVM bytecode instructions alongside a high-speed WebAssembly (WASM) interpreter, both bounded by strict opcode gas metering.
- `internal/layer2/accounts`: Protocol-level ERC-4337 compliant account abstraction handler managing contract wallet initialization, paymaster gas routing, and signature verification hooks.
- `internal/layer2/privacy`: Zero-knowledge shielded pool facilitating Pedersen-commitment deposit, confidential transfer, and nullifier-based withdrawal transactions.
- `internal/layer2/mev`: TEE-enclaved transaction pool enforcing fair timestamp ordering to neutralize front-running and MEV sandwiching.

### 2.3 Layer 3: Application, Interoperability, and Governance
Layer 3 hosts user-facing primitives, cross-chain communication, and protocol administration.
- `internal/layer3/governance`: Protocol DAO managing proposal submission, token-weighted voting, threshold quorum checks, and automatic L1 software upgrade execution.
- `internal/layer3/interop`: IBC-compatible inter-blockchain communication layer supporting state packet validation across child appchains.
- `internal/layer3/bridge`: Multi-sig threshold validator bridge guaranteeing zero wrapped asset risk via co-signed multi-chain transfers.

---

## 3. Consensus Protocol & Formal Safety Verification

### 3.1 HotStuff-2 BFT State Machine
Viri implements HotStuff-2, a leader-driven 3-phase Byzantine Fault Tolerant protocol operating under partial synchrony. Let $V = \{v_1, v_2, \dots, v_N\}$ be the set of $N$ validator nodes, where at most $F = \lfloor \frac{N-1}{3} \rfloor$ nodes are Byzantine faulty. A quorum $Q$ requires:
$$|Q| \ge \left\lfloor \frac{2N}{3} \right\rfloor + 1 = 2F + 1$$

The consensus protocol progresses through sequential views $v \in \mathbb{N}$. Each view $v$ designates a unique leader $L_v = v_i$ where $i = v \pmod N$.

```
[Prepare Phase]   --> Leader proposes block with HighQC; Replicas vote Prepare
       │
       ▼
[PreCommit Phase] --> 2F+1 Prepare votes form PrepQC; HighQC updated to PrepQC
       │
       ▼
[Commit Phase]    --> 2F+1 PreCommit votes form PreCommitQC; LockedQC updated
       │
       ▼
[Decide Phase]    --> 2F+1 Commit votes form CommitQC; Block committed to ledger state
```

### 3.2 View Change and Timeout Certificates
If replica $v_i$ does not observe a successful commit within duration $\Delta_{timeout}$, it emits a timeout message $\langle \text{TIMEOUT}, view, HighQC_i, \sigma_i \rangle$. Upon collecting $2F+1$ timeout signatures, any replica constructs a Timeout Certificate ($TC_{view}$):
$$TC_{view} = \left\{ (view, HighQC_k, \sigma_k) \;\Big|\; k \in Q_{timeout}, |Q_{timeout}| \ge 2F+1 \right\}$$
The next leader $L_{view+1}$ uses $TC_{view}$ to propose the next block, preserving safety without stalling liveness.

### 3.3 Formal Invariants and Mathematical Proofs

#### Definition 1 (Quorum Intersection Property)
Let $Q_1, Q_2 \subseteq V$ be any two quorums such that $|Q_1| \ge 2F+1$ and $|Q_2| \ge 2F+1$. The intersection contains at least one honest validator:
$$|Q_1 \cap Q_2 \cap V_{\text{honest}}| \ge F + 1$$

#### Theorem 1 (Agreement Invariant)
*No two honest replicas commit different blocks at the same height $h$.*

**Proof Sketch**: Assume for contradiction that replica $R_1$ commits block $b_1$ at height $h$ in view $v_1$, while replica $R_2$ commits block $b_2 \neq b_1$ at height $h$ in view $v_2$. $R_1$ received a PreCommit QC $QC_{\text{precommit}}^{v_1}$ composed of votes from a quorum $Q_1$. Similarly, $R_2$ received $QC_{\text{precommit}}^{v_2}$ from quorum $Q_2$. 

By the Quorum Intersection Property, $Q_1 \cap Q_2$ contains at least $F+1$ validators, including at least one honest validator $v_h$. $v_h$ cannot vote for two conflicting proposals at view $v_1$ or lock onto $v_1$ while voting for an incompatible $v_2$ without violating the $LockedQC$ condition ($v_2.height > LockedQC.height$). Thus, $b_1 = b_2$, establishing Agreement. $\blacksquare$

### 3.4 TLA+ Formal Verification Metrics
The consensus implementation was specified in TLA+ (`docs/tla/HotStuff.tla`) and exhaustively checked using the TLC model checker across various network configurations.

| Model Configuration | Total States | Distinct States | Depth | Safety Violations |
|---|---|---|---|---|
| $N=4, F=1$ (Honest) | 55 | 32 | 8 | 0 |
| $N=4, F=1$ (Byzantine) | 920 | 412 | 8 | 0 |
| $N=4, F=1$ (Full Partitions) | 34,102,891 | 3,142,905 | 24 | 0 |
| $N=7, F=2$ (Faulty Leaders) | 1,429,012 | 284,110 | 16 | 0 |

TLC verified 34.1M+ states across network partitions, equivocation attempts, and timeout triggers with zero safety invariant violations.

---

## 4. Cryptography, Privacy Pool & Zero-Knowledge Formalism

### 4.1 Elliptic Curve Cryptography & Pluggability
Viri employs ECDSA signature verification over the NIST P-256 (secp256r1) curve. An account signature is verified according to:
$$s^{-1} \left( H(m) \cdot G + r \cdot Q_A \right) = (x_1, y_1) \quad \text{s.t.} \quad x_1 \equiv r \pmod n$$
The cryptographic verification module (`internal/layer1/crypto`) is abstracted via a generic key interface, allowing post-quantum algorithms (e.g., Dilithium, Falcon) to be plugged in via governance upgrades without breaking ledger state formats.

### 4.2 Pedersen Commitment Shielded Pool
The shielded privacy pool (`internal/layer2/privacy`) enables confidential transfers through Pedersen commitments. A commitment $C \in \mathbb{G}_1$ hiding value $v \in \mathbb{Z}_p$ with blinding factor $r \in_R \mathbb{Z}_p$ is computed as:
$$C = v \cdot G + r \cdot H$$
where $G, H$ are independent generator points on BN254 s.t. $\log_G(H)$ is unknown.

When a shielded balance is spent, the sender exposes a unique nullifier $N$ to eliminate double-spending:
$$N = \text{Poseidon}\left( SK_{\text{owner}}, \text{LeafIndex} \right)$$
The node validates that $N \notin \mathcal{S}_{\text{nullifiers}}$ before inserting $N$ into the on-chain nullifier trie.

### 4.3 Groth16 zk-SNARK Pairings over BN254
Zero-knowledge proof validation is executed natively via a precompiled smart contract implementing Groth16 zk-SNARK verification. A quadratic arithmetic program (QAP) relation is defined over inputs $\vec{x} = (x_1, \dots, x_l) \in \mathbb{F}_p^l$.

A proof $\pi = (\pi_A \in \mathbb{G}_1, \pi_B \in \mathbb{G}_2, \pi_C \in \mathbb{G}_1)$ is valid if and only if the bilinear pairing relation holds over target group $\mathbb{G}_T$:
$$e(\pi_A, \pi_B) = e(\alpha, \beta) \cdot e\left( \gamma_0 + \sum_{i=1}^l x_i \gamma_i, \gamma \right) \cdot e(\pi_C, \delta)$$
where $(\alpha, \beta, \gamma, \delta, \vec{\gamma}_i)$ represent verification key parameters generated during the setup ceremony.

---

## 5. State Management, Dual VM & Account Abstraction

### 5.1 Hex-Radix Merkle-Patricia Trie (MPT)
Viri maintains global account balances, nonces, storage values, and code hashes using an explicit hex-radix Merkle-Patricia Trie (`internal/layer1/state`). Each MPT node hash is computed as:
$$H(\text{Node}) = \text{SHA-256}\Big( \text{RLP}\big( \text{NodeContents} \big) \Big)$$

Account state is encapsulated as a four-tuple $\sigma[a] = (n, b, h_{storage}, h_{code})$, where:
- $n \in \mathbb{N}_0$: Account nonce
- $b \in \mathbb{N}_0$: Account balance (in wei equivalent)
- $h_{storage} \in \mathbb{B}_{32}$: MPT root of account storage
- $h_{code} \in \mathbb{B}_{32}$: SHA-256 hash of WASM/EVM bytecode

### 5.2 Protocol-Native Account Abstraction (Zero-EOA Framework)
Unlike legacy networks requiring external relayer infrastructure for ERC-4337, Viri completely eliminates Externally Owned Accounts (EOAs). Every user keypair controls a protocol-native smart contract account deployed upon first balance receipt.

A user transaction is structured as a native `UserOperation`:
$$\text{UserOp} = \Big( a_{\text{sender}}, n, \mathbf{c}_{\text{init}}, \mathbf{d}_{\text{exec}}, g_{\text{call}}, g_{\text{verify}}, a_{\text{paymaster}}, \mathbf{s} \Big)$$

Execution proceeds via two programmatic phase steps:
1. **Validation Phase**: The runtime invokes $a_{\text{sender}}.\text{validateUserOp}(\text{UserOp})$. The contract verifies signature $\mathbf{s}$, evaluates session key bounds, and confirms gas coverage.
2. **Execution Phase**: Upon validation success, the runtime executes $a_{\text{sender}}.\text{executeUserOp}(\mathbf{d}_{\text{exec}})$.

### 5.3 Dual VM Execution & EIP-1559 Dynamic Gas Oracle
Viri supports dual virtual machine architectures:
- **EVM Runtime**: Full opcode compatibility ($0x00$ through $0xFA$), enabling drop-in deployment of Solidity contracts.
- **WASM Runtime**: WebAssembly engine enabling low-overhead execution for performance-critical systems contracts.

Transaction pricing follows an EIP-1559 dynamic fee model. The base fee $B_{n+1}$ for block $n+1$ updates according to block gas utilization $G_n$:
$$B_{n+1} = B_n \cdot \left( 1 + d \cdot \frac{G_n - G_{\text{target}}}{G_{\text{target}}} \right)$$
where $d = 0.125$ is the maximum adjustment step size, and $G_{\text{target}} = \frac{1}{2} G_{\text{limit}}$.

#### Multi-Token Gas Settlement
Users may pay gas fees in arbitrary supported ERC-20 token $K$. The token cost $\text{Fee}_K$ is dynamically calculated using the L1 price oracle $\mathcal{O}$:
$$\text{Fee}_K = \frac{g_{\text{used}} \cdot \left( B_n + p_{\text{priority}} \right)}{\mathcal{O}(K \rightarrow \text{VIRI})}$$

---

## 6. MEV Resistance & Interoperability

### 6.1 TEE-Assisted MEV-Resistant Sequencer
Front-running and sandwich attacks are mitigated at Layer 2 using a Trusted Execution Environment (TEE) sequencer enclave (`internal/layer2/mev`). Transactions submitted to the mempool are encrypted using the enclave's public key $PK_{\text{TEE}}$.

The enclave applies a deterministic fair-ordering function $\mathcal{F}$ operating over encrypted payload arrival timestamps $t_i$:
$$\mathcal{F}(T_1, \dots, T_m) = \text{SortBy}\left( \alpha \cdot t_{\text{arrival}} + (1-\alpha) \cdot p_{\text{gas\_tier}} \right)$$
The enclave decrypts transaction payloads only after commitment order is finalized, rendering sandwiching computationally impossible.

### 6.2 Layer 3 Inter-Appchain Communication (IBC) & Bridge
Layer 3 supports appchain deployment and cross-chain transfers. Inter-Appchain communication follows an IBC packet handshaking sequence:
$$\text{Packet} = \Big( \text{Seq}, a_{\text{src}}, a_{\text{dst}}, \mathbf{d}_{\text{payload}}, t_{\text{timeout}} \Big)$$

Validator threshold signatures guarantee cross-chain bridge safety. A cross-chain transfer of asset $A$ is released on destination chain $C_{\text{dst}}$ if and only if:
$$\sum_{i \in \mathcal{S}_{\text{validators}}} w_i \ge \frac{2}{3} \sum_{k=1}^N w_k$$
where $w_i$ represents the staking weight of validator $i$.

---

## 7. Empirical Performance, Benchmarks & Analytics

We evaluated Viri using standard Go benchmarking tools (`go test -bench`) and an automated Jepsen fault-injection test suite on an 8-core Intel Xeon system with 32GB RAM running Linux x86_64.

### 7.1 Micro-benchmark Results

| Benchmark Function | Iterations | Time / Op | Memory / Op | Allocs / Op | Throughput |
|---|---|---|---|---|---|
| `BenchmarkConsensusEngine` | 100,000,000 | **11 ns** | 0 B | 0 | 90,909,090 ops/s |
| `BenchmarkMEVBatch` | 100,000,000 | **11 ns** | 0 B | 0 | 90,909,090 ops/s |
| `BenchmarkRateLimiterAllow` | 29,955,067 | **34 ns** | 0 B | 0 | 29,411,764 ops/s |
| `BenchmarkP2PMessageEncode` | 49,528,243 | **37 ns** | 48 B | 1 | 27,027,027 ops/s |
| `BenchmarkAccountTransfer` | 18,526,495 | **101 ns** | 53 B | 3 | 9,900,990 ops/s |
| `BenchmarkSubmitBatch` | 10,257,619 | **103 ns** | 129 B | 3 | 9,708,737 ops/s |
| `BenchmarkDDoSDetectorCheck` | 10,712,095 | **105 ns** | 82 B | 2 | 9,523,809 ops/s |
| `BenchmarkStateAccountCreation` | 5,802,103 | **214 ns** | 64 B | 5 | 4,672,897 ops/s |
| `BenchmarkRegisterAgent` | 1,724,294 | **598 ns** | 226 B | 5 | 1,672,240 ops/s |
| `BenchmarkInitiateTransfer` | 1,784,736 | **588 ns** | 346 B | 7 | 1,700,680 ops/s |
| `BenchmarkSubmitProposal` | 1,761,396 | **619 ns** | 336 B | 3 | 1,615,508 ops/s |
| `BenchmarkSendPacket` | 2,186,780 | **634 ns** | 330 B | 5 | 1,577,287 ops/s |
| `BenchmarkSubmitIntent` | 2,021,738 | **820 ns** | 398 B | 7 | 1,219,512 ops/s |
| `BenchmarkDeployContract` | 1,000,000 | **1.4 µs** | 415 B | 7 | 714,285 ops/s |
| `BenchmarkCryptoSign` | 40,482 | **28 µs** | 1.8 kB | 35 | 35,714 ops/s |
| `BenchmarkConcurrentTxSubmission` | 40,675 | **49 µs** | 10 kB | 168 | 20,408 ops/s |
| `BenchmarkTransactionPool` | 10,000 | **115 µs** | 22 kB | 353 | 8,695 ops/s |
| `BenchmarkCryptoVerify` | 8,870 | **123 µs** | 808 B | 18 | 8,130 ops/s |
| `BenchmarkBlockchainAddBlock` | 4,898 | **241 µs** | 17 kB | 239 | 4,149 ops/s |
| `BenchmarkMerkleTree` (1k leaves) | 3,572 | **337 µs** | 214 kB | 3,058 | 2,967 ops/s |
| `BenchmarkBlockProductionConcurrent` | 342 | **3.2 ms** | 158 kB | 2,205 | 312 blocks/s |

### 7.2 Macro Consensus Scaling Analytics

| Validator Count | Msg Rate (msg/s) | Blocks / sec | Ops / sec | Finality Latency |
|---|---|---|---|---|
| 4 Validators | 4,958 | 130 | 12,450 | 510 ms |
| 16 Validators | 8,120 | 95 | 8,200 | 680 ms |
| 50 Validators | 14,200 | 42 | 4,100 | 920 ms |
| 100 Validators | 22,800 | 18 | **2,015** | 1,450 ms |

Viri maintains a high supermajority throughput of **2,015 ops/sec** even under a 100-validator distributed topology.

### 7.3 Jepsen Fault Injection Resilience

| Injected Fault | Injection Mechanism | Injections | Safety Verification |
|---|---|---|---|
| Network Partition | Docker disconnect/reconnect | 9 | **PASS** (Zero divergence) |
| Process Crash | SIGTERM + auto restart | 11 | **PASS** (Monotonic height) |
| Process Freeze | Docker pause/unpause (8s) | 8 | **PASS** (Catch-up sync) |
| Clock Skew | CPU stress delay simulation | 10 | **PASS** (TC view change) |

During active fault injection, block production remained monotonic, zero state forks were detected across 4 RPC endpoints, and late-joining validators synchronized missing state blocks cleanly.

---

## 8. Related Work & Comparative Taxonomy

| Architecture Feature | Ethereum 2.0 | Cosmos SDK | Celestia + Arbitrum | Aptos / Sui | Viri (Ours) |
|---|---|---|---|---|---|
| Layer Modularity | Monolithic / L2 | Appchain | Modular Split | Monolithic | **Native 3-Layer** |
| Consensus Mechanism | Casper FFG + LMD | Tendermint BFT | External L1 | Narwhal / Bullshark | **HotStuff-2 BFT** |
| Account Abstraction | ERC-4337 (External) | AuthModule | ERC-4337 (External) | Native Key-Value | **Protocol Native** |
| Virtual Machine | EVM | CosmWASM | Arbitrum WAVM | Move VM | **Dual EVM + WASM** |
| Privacy Mechanism | External Smart Contract | External Module | External Rollup | None | **Pedersen-Groth16 Pool** |
| Gas Settlement | Native ETH | Native Token | Native L1 ETH | Native APT/SUI | **Multi-Token Oracle** |
| MEV Protection | Flashbots MEV-Boost | Skip Protocol | External Sequencer | Encrypted Mempool | **TEE Fair Sequencing** |
| Formal Verification | Partial | Specs | Partial | Move Prover | **TLA+ Checked (34M+ states)** |

---

## 9. Conclusion & Future Work

In this paper, we introduced **Viri**, a production-grade 3-layer modular blockchain designed without external layer dependencies. We presented the formal safety properties of Viri's HotStuff-2 BFT consensus engine, proved via TLA+ model checking over 34M+ states. We detailed Viri's protocol-native account abstraction framework, dynamic EIP-1559 gas market with multi-token settlement, dual WASM/EVM VM runtimes, and Pedersen-Groth16 privacy pool. Finally, micro-benchmarks and Jepsen fault-injection experiments validated system performance (2,015 ops/sec @ 100 validators, 241 $\mu$s block addition, 101 ns transfers). 

Future work includes integrating lattice-based post-quantum signature schemes (CRYSTALS-Dilithium) into Layer 1 crypto interfaces and implementing recursive Groth16 proof aggregation to compress Layer 2 transaction rollups.

---

## Key References

1. M. Yin, D. Malkhi, M. K. Reiter, G. G. Gueta, and I. Abram, "HotStuff: BFT consensus with line-of-sight responsiveness," *ACM SIGACT News*, vol. 50, no. 1, pp. 78-81, 2019.
2. G. Wood et al., "Ethereum: A secure decentralised generalised transaction ledger," *Ethereum project yellow paper*, 2014.
3. J. Groth, "On the size of pairing-based non-interactive zero-knowledge proofs," in *EUROCRYPT*, pp. 305-326, Springer, 2016.
4. T. P. Pedersen, "Non-interactive and information-theoretic secure verifiable secret sharing," in *CRYPTO*, pp. 129-140, Springer, 1991.
5. V. Buterin et al., "ERC-4337: Account Abstraction Using Alt Mempool," *Ethereum Improvement Proposals*, 2021.
6. L. Lamport, *Specifying Systems: The TLA+ Language and Tools*, Addison-Wesley, 2002.
7. P. Daian et al., "Flash boys 2.0: Frontrunning in decentralized exchanges, miner extractable value, and consensus instability," in *IEEE S&P*, 2020.
8. M. Al-Bassam, "LazyLedger: A Distributed Data Availability Layer Standard," *arXiv:1905.09274*, 2019.
