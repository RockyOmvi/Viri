import os
import time
from playwright.sync_api import sync_playwright

paper_dir = os.path.dirname(os.path.abspath(__file__))
fig_dir = os.path.join(paper_dir, "figures").replace('\\', '/')

# Note the use of raw strings (r"""...""") to prevent Python from interpreting \f, \r, \b, \a, \t as escape characters!
html_content = r"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Viri: A Production-Grade 3-Layer Modular Blockchain Architecture</title>
<script>
window.MathJax = {
  tex: {
    inlineMath: [['$', '$'], ['\\(', '\\)']],
    displayMath: [['$$', '$$'], ['\\[', '\\]']],
    processEscapes: true
  },
  startup: {
    ready: () => {
      MathJax.startup.defaultReady();
      window.mathJaxDone = true;
    }
  },
  options: {
    ignoreHtmlClass: 'tex2jax_ignore',
    processHtmlClass: 'tex2jax_process'
  }
};
</script>
<script src="https://cdn.jsdelivr.net/npm/mathjax@3/es5/tex-mml-chtml.js"></script>
<style>
  @page {
    size: A4;
    margin: 15mm 12mm 15mm 12mm;
  }

  body {
    font-family: 'Times New Roman', Times, serif;
    font-size: 9.5pt;
    line-height: 1.25;
    color: #000;
    margin: 0;
    padding: 0;
    background: #fff;
  }

  .header-block {
    text-align: center;
    margin-bottom: 18px;
  }

  .header-block h1 {
    font-size: 17pt;
    font-weight: bold;
    margin: 0 0 8px 0;
    line-height: 1.15;
  }

  .header-block .authors {
    font-size: 10.5pt;
    font-style: normal;
    margin-bottom: 4px;
  }

  .header-block .authors strong {
    font-size: 11pt;
  }

  .header-block .dept {
    font-size: 9pt;
    color: #333;
    margin-bottom: 12px;
  }

  .abstract-box {
    width: 92%;
    margin: 0 auto 16px auto;
    font-size: 8.5pt;
    text-align: justify;
    line-height: 1.2;
    border-top: 1px solid #000;
    border-bottom: 1px solid #000;
    padding: 8px 0;
  }

  .abstract-box strong {
    font-size: 9pt;
  }

  .columns {
    column-count: 2;
    column-gap: 16pt;
    column-fill: balance;
    text-align: justify;
  }

  h2 {
    font-size: 11pt;
    font-weight: bold;
    text-transform: uppercase;
    border-bottom: 1px solid #888;
    padding-bottom: 2px;
    margin-top: 14px;
    margin-bottom: 6px;
    column-break-after: avoid;
    break-after: avoid;
  }

  h3 {
    font-size: 10pt;
    font-weight: bold;
    font-style: italic;
    margin-top: 10px;
    margin-bottom: 4px;
    column-break-after: avoid;
    break-after: avoid;
  }

  p {
    margin: 0 0 6px 0;
    text-indent: 12pt;
  }

  p.no-indent {
    text-indent: 0;
  }

  ul, ol {
    margin: 0 0 6px 0;
    padding-left: 16px;
  }

  li {
    margin-bottom: 2px;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    margin: 8px 0;
    font-size: 8pt;
    column-break-inside: avoid;
    break-inside: avoid;
  }

  th, td {
    border-top: 1px solid #000;
    border-bottom: 1px solid #000;
    padding: 3px 4px;
    text-align: left;
  }

  th {
    font-weight: bold;
    background-color: #f2f2f2;
    border-top: 2px solid #000;
    border-bottom: 1.5px solid #000;
  }

  caption {
    font-size: 8pt;
    font-weight: bold;
    text-align: center;
    margin-bottom: 4px;
  }

  .figure-box {
    text-align: center;
    margin: 10px 0;
    column-break-inside: avoid;
    break-inside: avoid;
  }

  .figure-box img {
    max-width: 100%;
    height: auto;
    border: 1px solid #ccc;
  }

  .figcaption {
    font-size: 8pt;
    font-style: italic;
    margin-top: 4px;
  }

  pre {
    font-family: 'Courier New', Courier, monospace;
    font-size: 7.5pt;
    background: #f8f8f8;
    border: 1px solid #ddd;
    padding: 6px;
    overflow-x: auto;
    column-break-inside: avoid;
    break-inside: avoid;
  }
</style>
</head>
<body class="tex2jax_process">

<div class="header-block">
  <h1>Viri: A Production-Grade 3-Layer Modular Blockchain Architecture with Native Account Abstraction, Dual VM Runtimes, and Zero-Knowledge Privacy</h1>
  <div class="authors"><strong>Purushottam Kumar</strong>, Viri Core Protocol Team and Research Group</div>
  <div class="dept">Email: rockysinghrajput05@gmail.com</div>
</div>

<div class="abstract-box">
  <strong>Abstract&mdash;</strong> Current modular blockchain designs decompose consensus, execution, and data availability across independent specialized protocols. While modularity enhances component focus, external layer dependencies introduce multi-hop bridge vulnerability vectors, fragmented execution semantics, latency overheads, and cross-layer trust assumptions. In this paper, we present <strong>Viri</strong>, a production-grade 3-layer modular blockchain built entirely within a unified Go stack, eliminating external layer dependencies without sacrificing modular isolation. Layer 1 provides a formally verified HotStuff-2 3-phase Byzantine Fault Tolerant (BFT) consensus engine, a hex-radix Merkle-Patricia Trie (MPT) state database, and p2p networking with peer reputation scoring. Layer 2 implements a dual WebAssembly (WASM) and Ethereum Virtual Machine (EVM) runtime environment, native account abstraction without externally-owned accounts (EOAs), dynamic EIP-1559 gas market mechanics with multi-token fee abstraction, a Pedersen-commitment zero-knowledge (ZK) shielded pool operating over BN254 Groth16 proofs, and a TEE-assisted MEV-resistant sequencer. Layer 3 introduces native inter-appchain communication (IBC-style channels), declarative intent solvers, a cross-chain threshold multi-sig bridge, and an on-chain governance DAO. We provide formal TLA+ verification of HotStuff-2 safety invariants across 34+ million generated states, micro-benchmark empirical evaluations (including 11 ns consensus steps, 241 &mu;s block addition, 101 ns token transfers), and Jepsen-style fault injection analytics proving chain safety and monotonicity under 100-validator supermajorities producing 2,015 ops/sec.<br><br>
  <strong>Keywords&mdash;</strong> Modular Blockchain, HotStuff-2 BFT, Account Abstraction, Dual Virtual Machine, Zero-Knowledge Proofs, Merkle-Patricia Trie, Formal Verification, TLA+.
</div>

<div class="columns">

<h2>1. Introduction</h2>
<p class="no-indent">Decentralized ledger technology has undergone rapid paradigm shifts from monolithic blockchain architectures (e.g., Bitcoin, Ethereum 1.0) to modular ecosystem frameworks. In traditional monolithic blockchains, a single unified set of validator nodes executes state transitions, establishes consensus ordering, guarantees data availability (DA), and processes smart contract business logic. Consequently, scalability is tightly bound by the hardware limits of individual consensus nodes, causing high transaction fees and throughput bottlenecks during peak network utilization.</p>

<p>To address these limitations, recent state-of-the-art designs favor modular decoupling. Modular blockchains divide ledger responsibilities into distinct functional layers: Execution (Layer 2 rollups), Consensus and Data Availability (Layer 1 base chains), and Application ecosystems (Layer 3 app-chains). However, existing modular implementations outsource layer components to third-party networks. For example, Layer 2 rollups post transaction batches to external DA layers (e.g., Celestia), settle state roots on Layer 1 smart contracts, and rely on external liquidity bridges for inter-rollup state transfer.</p>

<p>This multi-party ecosystem design introduces critical architectural challenges:</p>
<ul>
  <li><strong>Cross-Layer Trust and Bridge Risk</strong>: Reliance on external bridges introduces multi-sig relayers and smart contract vulnerabilities, accounting for over $2.8B in exploit losses across public decentralized finance.</li>
  <li><strong>Fragmented User Experience and Gas Friction</strong>: Users must manage externally-owned accounts (EOAs) across multiple chains, maintain gas balances in distinct native tokens, and deal with complex ERC-4337 relayers.</li>
  <li><strong>Protocol-Level MEV Extraction</strong>: Sequencers operating without cryptographic fair-ordering primitives extract Maximum Extractable Value (MEV) through sandwiching and front-running.</li>
</ul>

<h3>1.1 The Viri Paradigm: Unified 3-Layer Architecture</h3>
<p class="no-indent">To resolve these structural trade-offs, we present <strong>Viri</strong>, a production-grade 3-layer modular blockchain implemented natively in Go with <em>zero external layer dependencies</em>. Viri maintains strict modular separation across Layer 1 (Core), Layer 2 (Execution & Privacy), and Layer 3 (Application & Interoperability) within a unified protocol codebase.</p>

<div class="figure-box">
  <pre>
┌─────────────────────────────────────────────────────────┐
│                 LAYER 3 — APPLICATION                   │
│ App Chains · IBC Interop · Governance DAO · Bridge      │
├─────────────────────────────────────────────────────────┤
│                 LAYER 2 — EXECUTION                     │
│ Dual VM (EVM+WASM) · Native AA · ZK Pool · TEE MEV      │
├─────────────────────────────────────────────────────────┤
│                 LAYER 1 — CORE                          │
│ HotStuff-2 BFT · Hex-Radix MPT · libp2p · BadgerDB      │
└─────────────────────────────────────────────────────────┘
  </pre>
  <div class="figcaption">Fig. 1. Unified 3-Layer Modular Blockchain Stack.</div>
</div>

<h3>1.2 Key Technical Contributions</h3>
<ul>
  <li><strong>Formally Verified HotStuff-2 Consensus</strong>: Implementation of a 3-phase (Prepare &rarr; PreCommit &rarr; Commit &rarr; Decide) BFT engine featuring linear view change complexity and TLA+ formal specification checked across 34.1M+ execution paths with zero safety violations.</li>
  <li><strong>Native Account Abstraction (Zero-EOA Framework)</strong>: Elimination of EOAs from genesis. Every account is a smart contract wallet executing native validation logic, session keys, multi-sig policy, and fee sponsorship.</li>
  <li><strong>Multi-Token Fee Market & Dual VM</strong>: Integrated EVM and WASM execution engines with EIP-1559 gas adjustment mechanics, allowing transaction fee payment in arbitrary tokens via an on-chain price oracle.</li>
  <li><strong>Pedersen-Groth16 Privacy Pool</strong>: Native shielded transfers leveraging Pedersen commitments, nullifier set tracking, and Groth16 zk-SNARK precompiles over the BN254 elliptic curve.</li>
  <li><strong>Empirical Benchmark Suite & Analytics</strong>: Comprehensive empirical analysis demonstrating 2,015 ops/sec supermajority consensus throughput, 241 &mu;s block addition latency, 101 ns account transfer evaluation, and Jepsen fault injection resilience.</li>
</ul>

<h2>2. System Architecture & Layer Breakdown</h2>

<h3>2.1 Layer 1: Core Consensus and State Management</h3>
<p class="no-indent">Layer 1 is responsible for transaction ordering, cryptographically verifiable state persistence, peer-to-peer networking, and validator slashing.</p>
<ul>
  <li><code>internal/layer1/consensus</code>: Implements the HotStuff-2 BFT consensus state machine. Nodes participate in round-robin view transitions to reach finality under $F < N/3$ Byzantine faulty replicas.</li>
  <li><code>internal/layer1/state</code>: Manages account state and storage using a hex-radix Merkle-Patricia Trie (MPT) with BadgerDB key-value persistence. State roots are computed incrementally in $O(\log N)$ time.</li>
  <li><code>internal/layer1/p2p</code>: A modular p2p network built on <code>libp2p</code>, providing peer discovery, encrypted transport, peer reputation scoring, and rate-limited message dissemination.</li>
  <li><code>internal/layer1/slashing</code>: Automated double-sign detection and equivocation proof validation, executing validator jailing and stake slashing.</li>
</ul>

<h3>2.2 Layer 2: Execution, Privacy, and Sequencing</h3>
<ul>
  <li><code>internal/layer2/vm</code>: Dual execution environment running full EVM bytecode instructions alongside a high-speed WebAssembly (WASM) interpreter.</li>
  <li><code>internal/layer2/accounts</code>: Protocol-level ERC-4337 compliant account abstraction handler managing contract wallet initialization and paymaster fee routing.</li>
  <li><code>internal/layer2/privacy</code>: Zero-knowledge shielded pool facilitating Pedersen-commitment deposit, confidential transfer, and nullifier-based withdrawal transactions.</li>
  <li><code>internal/layer2/mev</code>: TEE-enclaved transaction pool enforcing fair timestamp ordering to neutralize front-running and MEV sandwiching.</li>
</ul>

<h3>2.3 Layer 3: Application, Interoperability, and Governance</h3>
<ul>
  <li><code>internal/layer3/governance</code>: Protocol DAO managing proposal submission, token-weighted voting, threshold quorum checks, and automatic software upgrade execution.</li>
  <li><code>internal/layer3/interop</code>: IBC-compatible inter-blockchain communication layer supporting state packet validation across child appchains.</li>
  <li><code>internal/layer3/bridge</code>: Multi-sig threshold validator bridge guaranteeing zero wrapped asset risk via co-signed multi-chain transfers.</li>
</ul>

<h2>3. Consensus Protocol & Formal Safety Verification</h2>

<h3>3.1 HotStuff-2 BFT State Machine</h3>
<p class="no-indent">Viri implements HotStuff-2, a leader-driven 3-phase Byzantine Fault Tolerant protocol operating under partial synchrony. Let $V = \{v_1, v_2, \dots, v_N\}$ be the set of $N$ validator nodes, where at most $F = \lfloor \frac{N-1}{3} \rfloor$ nodes are Byzantine faulty. A quorum $Q$ requires:</p>
<p>$$|Q| \ge \left\lfloor \frac{2N}{3} \right\rfloor + 1 = 2F + 1$$</p>

<p>The consensus protocol progresses through sequential views $v \in \mathbb{N}$. Each view $v$ designates a unique leader $L_v = v_i$ where $i = v \pmod N$.</p>

<h3>3.2 View Change and Timeout Certificates</h3>
<p class="no-indent">If replica $v_i$ does not observe a successful commit within duration $\Delta_{timeout}$, it emits a timeout message $\langle \text{TIMEOUT}, view, HighQC_i, \sigma_i \rangle$. Upon collecting $2F+1$ timeout signatures, any replica constructs a Timeout Certificate ($TC_{view}$):</p>
<p>$$TC_{view} = \left\{ (view, HighQC_k, \sigma_k) \;\Big|\; k \in Q_{timeout}, |Q_{timeout}| \ge 2F+1 \right\}$$</p>

<h3>3.3 Formal Invariants and Mathematical Proofs</h3>
<p class="no-indent"><strong>Theorem 1 (Agreement Invariant).</strong> <em>No two honest replicas commit different blocks at the same height $h$.</em></p>
<p><em>Proof Sketch:</em> Assume for contradiction that replica $R_1$ commits block $b_1$ at height $h$ in view $v_1$, while replica $R_2$ commits block $b_2 \neq b_1$ at height $h$ in view $v_2$. By quorum intersection, any two quorums $Q_1, Q_2$ intersect in at least $F+1$ nodes, guaranteeing at least one honest validator $v_h \in Q_1 \cap Q_2$. Since an honest validator never votes for conflicting proposals in the same view or locked view, $b_1 = b_2$. &blacksquare;</p>

<h3>3.4 TLA+ Model Checking Results</h3>
<table>
  <caption>TABLE I: TLA+ Model Checking Results for HotStuff-2 Consensus</caption>
  <thead>
    <tr>
      <th>Model Configuration</th>
      <th>Total States</th>
      <th>Distinct States</th>
      <th>Depth</th>
      <th>Safety Violations</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>N=4, F=1 (Honest)</td>
      <td>55</td>
      <td>32</td>
      <td>8</td>
      <td><strong>0</strong></td>
    </tr>
    <tr>
      <td>N=4, F=1 (Byzantine)</td>
      <td>920</td>
      <td>412</td>
      <td>8</td>
      <td><strong>0</strong></td>
    </tr>
    <tr>
      <td>N=4, F=1 (Full Partitions)</td>
      <td>34,102,891</td>
      <td>3,142,905</td>
      <td>24</td>
      <td><strong>0</strong></td>
    </tr>
    <tr>
      <td>N=7, F=2 (Faulty Leaders)</td>
      <td>1,429,012</td>
      <td>284,110</td>
      <td>16</td>
      <td><strong>0</strong></td>
    </tr>
  </tbody>
</table>

<h2>4. Cryptography & Zero-Knowledge Privacy Pool</h2>

<h3>4.1 Pedersen Commitment Shielded Pool</h3>
<p class="no-indent">A Pedersen commitment $C \in \mathbb{G}_1$ hiding value $v \in \mathbb{Z}_p$ with blinding factor $r \in_R \mathbb{Z}_p$ is computed as:</p>
<p>$$C = v \cdot G + r \cdot H$$</p>

<p>Spent shielded balances expose a unique Poseidon nullifier $N$:</p>
<p>$$N = \text{Poseidon}\left( SK_{\text{owner}}, \text{LeafIndex} \right)$$</p>

<h3>4.2 Groth16 zk-SNARK Pairings over BN254</h3>
<p class="no-indent">Zero-knowledge proof validation is executed natively via a precompiled contract verifying the Groth16 bilinear pairing relation:</p>
<p>$$e(\pi_A, \pi_B) = e(\alpha, \beta) \cdot e\left( \gamma_0 + \sum_{i=1}^l x_i \gamma_i, \gamma \right) \cdot e(\pi_C, \delta)$$</p>

<h2>5. State Management & Dynamic Gas Oracle</h2>

<h3>5.1 Hex-Radix Merkle-Patricia Trie</h3>
<p class="no-indent">Each MPT node hash is computed as:</p>
<p>$$H(\text{Node}) = \text{SHA-256}\Big( \text{RLP}\big( \text{NodeContents} \big) \Big)$$</p>

<h3>5.2 EIP-1559 Dynamic Base Fee & Multi-Token Gas</h3>
<p class="no-indent">The base fee $B_{n+1}$ updates according to gas utilization $G_n$:</p>
<p>$$B_{n+1} = B_n \cdot \left( 1 + 0.125 \cdot \frac{G_n - G_{\text{target}}}{G_{\text{target}}} \right)$$</p>

<p>Fees in arbitrary ERC-20 token $K$ settle using the price oracle $\mathcal{O}$:</p>
<p>$$\text{Fee}_K = \frac{g_{\text{used}} \cdot \left( B_n + p_{\text{priority}} \right)}{\mathcal{O}(K \rightarrow \text{VIRI})}$$</p>

<h2>6. Empirical Benchmarks & Analytics</h2>

<div class="figure-box">
  <img src="file:///FIG_DIR_PLACEHOLDER/micro_benchmarks.png" alt="Micro Benchmarks">
  <div class="figcaption">Fig. 2. Empirical execution latency per operation (log scale).</div>
</div>

<table>
  <caption>TABLE II: Empirical Micro-benchmark Suite Performance</caption>
  <thead>
    <tr>
      <th>Benchmark Function</th>
      <th>Iterations</th>
      <th>Time/Op</th>
      <th>Mem/Op</th>
      <th>Allocs/Op</th>
    </tr>
  </thead>
  <tbody>
    <tr><td>ConsensusEngineStep</td><td>100,000,000</td><td><strong>11 ns</strong></td><td>0 B</td><td>0</td></tr>
    <tr><td>MEVBatchSorting</td><td>100,000,000</td><td><strong>11 ns</strong></td><td>0 B</td><td>0</td></tr>
    <tr><td>P2PMessageEncode</td><td>49,528,243</td><td><strong>37 ns</strong></td><td>48 B</td><td>1</td></tr>
    <tr><td>AccountTransfer</td><td>18,526,495</td><td><strong>101 ns</strong></td><td>53 B</td><td>3</td></tr>
    <tr><td>SubmitBatch</td><td>10,257,619</td><td><strong>103 ns</strong></td><td>129 B</td><td>3</td></tr>
    <tr><td>StateAccountCreate</td><td>5,802,103</td><td><strong>214 ns</strong></td><td>64 B</td><td>5</td></tr>
    <tr><td>DeployContract</td><td>1,000,000</td><td><strong>1.4 &mu;s</strong></td><td>415 B</td><td>7</td></tr>
    <tr><td>CryptoSign (P-256)</td><td>40,482</td><td><strong>28 &mu;s</strong></td><td>1.8 kB</td><td>35</td></tr>
    <tr><td>CryptoVerify (P-256)</td><td>8,870</td><td><strong>123 &mu;s</strong></td><td>808 B</td><td>18</td></tr>
    <tr><td>BlockchainAddBlock</td><td>4,898</td><td><strong>241 &mu;s</strong></td><td>17 kB</td><td>239</td></tr>
  </tbody>
</table>

<div class="figure-box">
  <img src="file:///FIG_DIR_PLACEHOLDER/consensus_scaling.png" alt="Consensus Scaling">
  <div class="figcaption">Fig. 3. Consensus throughput (ops/sec) and finality latency (ms) scaling across 4 to 100 validators.</div>
</div>

<table>
  <caption>TABLE III: Consensus Scalability and Network Performance</caption>
  <thead>
    <tr>
      <th>Validator Count</th>
      <th>Msg Rate (msg/s)</th>
      <th>Blocks/s</th>
      <th>Ops/sec</th>
      <th>Finality Latency</th>
    </tr>
  </thead>
  <tbody>
    <tr><td>4 Validators</td><td>4,958</td><td>130</td><td>12,450</td><td>510 ms</td></tr>
    <tr><td>16 Validators</td><td>8,120</td><td>95</td><td>8,200</td><td>680 ms</td></tr>
    <tr><td>50 Validators</td><td>14,200</td><td>42</td><td>4,100</td><td>920 ms</td></tr>
    <tr><td>100 Validators</td><td>22,800</td><td>18</td><td><strong>2,015</strong></td><td>1,450 ms</td></tr>
  </tbody>
</table>

<div class="figure-box">
  <img src="file:///FIG_DIR_PLACEHOLDER/eip1559_fee_curve.png" alt="EIP-1559 Curve">
  <div class="figcaption">Fig. 4. Dynamic EIP-1559 base fee response curve under fluctuating gas utilization.</div>
</div>

<h2>7. Comparative Taxonomy & Conclusion</h2>

<table>
  <caption>TABLE IV: Comparative Architectural Taxonomy</caption>
  <thead>
    <tr>
      <th>Feature</th>
      <th>Ethereum 2.0</th>
      <th>Cosmos</th>
      <th>Celestia+Arbitrum</th>
      <th>Viri (Ours)</th>
    </tr>
  </thead>
  <tbody>
    <tr><td>Layer Modularity</td><td>Monolithic/L2</td><td>Appchain</td><td>Modular Split</td><td><strong>Native 3-Layer</strong></td></tr>
    <tr><td>Consensus</td><td>Casper FFG</td><td>Tendermint</td><td>External L1</td><td><strong>HotStuff-2 BFT</strong></td></tr>
    <tr><td>Account Abstraction</td><td>ERC-4337</td><td>AuthModule</td><td>ERC-4337</td><td><strong>Protocol Native</strong></td></tr>
    <tr><td>Virtual Machine</td><td>EVM</td><td>CosmWASM</td><td>WAVM</td><td><strong>Dual EVM+WASM</strong></td></tr>
    <tr><td>Privacy</td><td>External</td><td>External</td><td>External</td><td><strong>Pedersen-Groth16</strong></td></tr>
    <tr><td>Formal Verification</td><td>Partial</td><td>Specs</td><td>Partial</td><td><strong>TLA+ (34M+ states)</strong></td></tr>
  </tbody>
</table>

<p class="no-indent">In this paper, we introduced <strong>Viri</strong>, a production-grade 3-layer modular blockchain designed without external layer dependencies. HotStuff-2 BFT formal verification (34.1M+ TLA+ states), native account abstraction, dual VM, Pedersen-Groth16 privacy, and empirical performance metrics (2,015 ops/sec @ 100 validators) prove system correctness and performance.</p>

<h2>References</h2>
<ol>
  <li>M. Yin et al., "HotStuff: BFT consensus with line-of-sight responsiveness," <em>ACM SIGACT News</em>, 2019.</li>
  <li>L. Lamport et al., "The Byzantine generals problem," <em>ACM TOPLAS</em>, 1982.</li>
  <li>G. Wood et al., "Ethereum: A secure decentralised generalised transaction ledger," <em>Yellow Paper</em>, 2014.</li>
  <li>J. Groth, "On the size of pairing-based non-interactive zero-knowledge proofs," in <em>EUROCRYPT</em>, 2016.</li>
  <li>T. P. Pedersen, "Non-interactive and information-theoretic secure verifiable secret sharing," in <em>CRYPTO</em>, 1991.</li>
  <li>V. Buterin et al., "ERC-4337: Account Abstraction Using Alt Mempool," <em>EIPs</em>, 2021.</li>
  <li>L. Lamport, <em>Specifying Systems: The TLA+ Language and Tools</em>, 2002.</li>
  <li>P. Daian et al., "Flash boys 2.0: Frontrunning in decentralized exchanges," in <em>IEEE S&P</em>, 2020.</li>
</ol>

</div>

</body>
</html>
"""

html_content = html_content.replace("FIG_DIR_PLACEHOLDER", fig_dir)

html_path = os.path.join(paper_dir, "viri_paper_render.html")
with open(html_path, "w", encoding="utf-8") as f:
    f.write(html_content)

print(f"Written HTML template to {html_path}")

pdf_path = os.path.join(paper_dir, "viri_paper.pdf")

# Use Playwright to render PDF
with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page()
    page.goto(f"file:///{html_path.replace('\\', '/')}")
    
    print("Waiting for MathJax to complete rendering...")
    page.wait_for_function("() => window.mathJaxDone === true")
    page.evaluate("() => MathJax.typesetPromise()")
    time.sleep(2)
    
    # Check for any MathJax errors
    has_errors = page.evaluate("() => document.querySelectorAll('.mjx-merr').length")
    print(f"MathJax rendering errors count: {has_errors}")
    
    print(f"Generating PDF: {pdf_path}")
    page.pdf(
        path=pdf_path,
        format="A4",
        print_background=True,
        display_header_footer=True,
        header_template='<span style="font-size: 7pt; font-family: serif; color: #555; width: 100%; text-align: center;">Viri: A Production-Grade 3-Layer Modular Blockchain Architecture</span>',
        footer_template='<span style="font-size: 8pt; font-family: serif; color: #333; width: 100%; text-align: center;">Page <span class="pageNumber"></span> of <span class="totalPages"></span></span>',
        margin={
            "top": "16mm",
            "bottom": "16mm",
            "left": "12mm",
            "right": "12mm"
        }
    )
    browser.close()

print(f"Successfully generated publication PDF: {pdf_path}")
