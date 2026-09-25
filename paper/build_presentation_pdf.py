import os
import time
from playwright.sync_api import sync_playwright

paper_dir = os.path.dirname(os.path.abspath(__file__))
fig_dir = os.path.join(paper_dir, "figures").replace('\\', '/')

slides_html = r"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Viri Blockchain Presentation</title>
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
    size: 16in 9in;
    margin: 0;
  }
  body {
    font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
    margin: 0;
    padding: 0;
    background: #f4f6f9;
    color: #222;
  }
  .slide {
    width: 16in;
    height: 9in;
    box-sizing: border-box;
    padding: 0.65in 0.9in;
    page-break-after: always;
    position: relative;
    background: #ffffff;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }
  .slide-header {
    border-bottom: 4px solid #1a365d;
    padding-bottom: 0.15in;
    margin-bottom: 0.25in;
  }
  .slide-header h2 {
    margin: 0;
    font-size: 34pt;
    color: #1a365d;
    font-weight: 700;
  }
  .slide-header p {
    margin: 4px 0 0 0;
    font-size: 17pt;
    color: #718096;
  }
  .slide-body {
    flex-grow: 1;
    font-size: 19pt;
    line-height: 1.45;
  }
  .slide-footer {
    border-top: 1px solid #e2e8f0;
    padding-top: 0.12in;
    display: flex;
    justify-content: space-between;
    font-size: 13pt;
    color: #a0aec0;
  }
  .title-slide {
    background: linear-gradient(135deg, #1a365d 0%, #2b6cb0 100%);
    color: white;
    justify-content: center;
    align-items: center;
    text-align: center;
  }
  .title-slide h1 {
    font-size: 50pt;
    margin-bottom: 12px;
    font-weight: 800;
  }
  .title-slide h3 {
    font-size: 26pt;
    color: #e2e8f0;
    font-weight: 400;
    margin-bottom: 35px;
  }
  .title-slide .authors {
    font-size: 21pt;
    color: #cbd5e0;
    line-height: 1.5;
  }
  .title-slide .authors strong {
    color: #ffffff;
    font-size: 24pt;
  }
  .card {
    background: #edf2f7;
    border-left: 6px solid #2b6cb0;
    padding: 16px 22px;
    border-radius: 6px;
    margin: 10px 0;
    font-size: 18pt;
  }
  .formula-card {
    background: #f7fafc;
    border: 1px solid #e2e8f0;
    border-left: 6px solid #319795;
    padding: 14px 20px;
    border-radius: 6px;
    margin: 12px 0;
    font-size: 19pt;
    text-align: center;
  }
  .two-cols {
    display: flex;
    gap: 30px;
  }
  .two-cols > div {
    flex: 1;
  }
  ul {
    margin-top: 6px;
    margin-bottom: 8px;
    padding-left: 28px;
  }
  li {
    margin-bottom: 8px;
  }
  .fig-center {
    text-align: center;
  }
  .fig-center img {
    max-height: 4.8in;
    max-width: 90%;
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 16pt;
    margin-top: 10px;
  }
  th, td {
    padding: 10px 14px;
    border-bottom: 1px solid #cbd5e0;
    text-align: left;
  }
  th {
    background-color: #2b6cb0;
    color: white;
  }
</style>
</head>
<body class="tex2jax_process">

<!-- Slide 1: Title -->
<div class="slide title-slide">
  <div>
    <h1>Viri Blockchain Architecture</h1>
    <h3>Native Account Abstraction, Dual VM & Zero-Knowledge Privacy</h3>
    <div class="authors">
      <strong>Purushottam Kumar</strong><br>
      Viri Core Protocol Team & Research Group<br>
      rockysinghrajput05@gmail.com &bull; August 2026 &bull; <span style="color: #cbd5e0;">https://viri.me</span>
    </div>
  </div>
</div>

<!-- Slide 2: Problem Statement -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>The Modular Blockchain Dilemma</h2>
      <p>Why existing modular architectures introduce critical systemic risk</p>
    </div>
    <div class="slide-body">
      <ul>
        <li><strong>Monolithic Bottlenecks</strong>: Single validator set handles execution, consensus, state, and DA &rarr; throughput bounded by single-node hardware limits.</li>
        <li><strong>Fragmented Modular Dependencies</strong>: Outsourcing DA to Celestia, execution to Arbitrum, settlement to Ethereum introduces multi-hop trust layers.</li>
        <li><strong>Key Vulnerability Vectors</strong>:
          <ul>
            <li><strong>Cross-Layer Bridge Risk</strong>: Over $2.8B exploited across third-party liquidity bridges.</li>
            <li><strong>UX & Gas Friction</strong>: Managing EOAs, fragmented gas tokens, ERC-4337 relayers.</li>
            <li><strong>MEV Extraction</strong>: Sequencers front-running & sandwiching unencrypted mempools.</li>
          </ul>
        </li>
      </ul>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 2</span>
  </div>
</div>

<!-- Slide 3: Unified Architecture -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>The Viri Unified 3-Layer Solution</h2>
      <p>Strict layer separation without external layer dependencies</p>
    </div>
    <div class="slide-body">
      <div class="card">
        <strong>Layer 1 (Core Infrastructure)</strong>: Formally verified HotStuff-2 BFT consensus, hex-radix MPT, BadgerDB persistence, libp2p authenticated networking.
      </div>
      <div class="card">
        <strong>Layer 2 (Execution & Privacy)</strong>: Dual VM (EVM + WASM), native account abstraction (zero EOAs), Pedersen-Groth16 ZK pool, TEE MEV resistance.
      </div>
      <div class="card">
        <strong>Layer 3 (Application Ecosystem)</strong>: On-chain Governance DAO, IBC-style inter-appchain channels, threshold validator bridge, intent solvers.
      </div>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 3</span>
  </div>
</div>

<!-- Slide 4: HotStuff-2 Consensus -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>Layer 1: HotStuff-2 BFT Consensus</h2>
      <p>3-phase leader-driven BFT state machine with optimistic responsiveness</p>
    </div>
    <div class="slide-body">
      <ul>
        <li><strong>Quorum Condition</strong>: Under $N \ge 3F + 1$, quorum threshold $|Q|$ satisfies:</li>
      </ul>
      <div class="formula-card">
        $$|Q| \ge \left\lfloor \frac{2N}{3} \right\rfloor + 1 = 2F + 1$$
      </div>
      <ul>
        <li><strong>3-Phase Pipeline</strong>: Linear view change complexity using Timeout Certificates ($TC$):</li>
      </ul>
      <div class="formula-card">
        $$\text{Prepare} \;\xrightarrow{QC_{\text{prep}}}\; \text{PreCommit} \;\xrightarrow{QC_{\text{precommit}}}\; \text{Commit} \;\xrightarrow{QC_{\text{commit}}}\; \text{Decide}$$
      </div>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 4</span>
  </div>
</div>

<!-- Slide 5: Formal Verification -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>TLA+ Formal Safety Verification</h2>
      <p>Exhaustive TLC model checking across 34.1M+ states</p>
    </div>
    <div class="slide-body">
      <table>
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
          <tr><td>$N=4, F=1$ (Honest)</td><td>55</td><td>32</td><td>8</td><td><strong>0</strong></td></tr>
          <tr><td>$N=4, F=1$ (Byzantine)</td><td>920</td><td>412</td><td>8</td><td><strong>0</strong></td></tr>
          <tr><td>$N=4, F=1$ (Full Partitions)</td><td>34,102,891</td><td>3,142,905</td><td>24</td><td><strong>0</strong></td></tr>
          <tr><td>$N=7, F=2$ (Faulty Leaders)</td><td>1,429,012</td><td>284,110</td><td>16</td><td><strong>0</strong></td></tr>
        </tbody>
      </table>
      <div class="card" style="margin-top: 15px;">
        <strong>Verified Invariants</strong>: Agreement, NoDoubleCommit, LockedViewInvariant, and QuorumIntersection hold across all Byzantine attack vectors.
      </div>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 5</span>
  </div>
</div>

<!-- Slide 6: Native Account Abstraction -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>Layer 2: Native Account Abstraction</h2>
      <p>Zero EOAs: Every account is a native smart contract wallet from genesis</p>
    </div>
    <div class="slide-body">
      <ul>
        <li><strong>Native UserOperation Pipeline</strong>:</li>
      </ul>
      <div class="formula-card">
        $$\text{UserOp} = \Big( a_{\text{sender}}, n, \mathbf{c}_{\text{init}}, \mathbf{d}_{\text{exec}}, g_{\text{call}}, g_{\text{verify}}, a_{\text{paymaster}}, \mathbf{s} \Big)$$
      </div>
      <div class="two-cols">
        <div class="card">
          <strong>1. Validation Phase</strong><br>
          Invokes $a_{\text{sender}}.\text{validateUserOp}()$. Validates session keys, social recovery signatures, and fee coverage.
        </div>
        <div class="card">
          <strong>2. Execution Phase</strong><br>
          Invokes $a_{\text{sender}}.\text{executeUserOp}()$. Executes contract call state transitions upon validation success.
        </div>
      </div>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 6</span>
  </div>
</div>

<!-- Slide 7: Dual VM & Gas Market -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>Dual VM & Dynamic Multi-Token Gas</h2>
      <p>Drop-in Solidity EVM + Wazero WASM runtime with dynamic EIP-1559 settlement</p>
    </div>
    <div class="slide-body">
      <div class="two-cols">
        <div>
          <div class="card"><strong>EIP-1559 Dynamic Base Fee</strong></div>
          <div class="formula-card">
            $$B_{n+1} = B_n \cdot \left( 1 + 0.125 \cdot \frac{G_n - G_{\text{target}}}{G_{\text{target}}} \right)$$
          </div>
        </div>
        <div>
          <div class="card"><strong>Multi-Token Gas Settlement</strong></div>
          <div class="formula-card">
            $$\text{Fee}_K = \frac{g_{\text{used}} \cdot \left( B_n + p_{\text{priority}} \right)}{\mathcal{O}(K \rightarrow \text{VIRI})}$$
          </div>
        </div>
      </div>
      <ul>
        <li>Pay transaction fees in USDC, USDT, or arbitrary ERC-20 via on-chain oracle conversion.</li>
      </ul>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 7</span>
  </div>
</div>

<!-- Slide 8: ZK Privacy Pool -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>Pedersen-Groth16 Shielded Privacy Pool</h2>
      <p>On-chain ZK precompile over BN254 elliptic curve using gnark</p>
    </div>
    <div class="slide-body">
      <div class="two-cols">
        <div>
          <div class="card"><strong>Pedersen Commitment</strong></div>
          <div class="formula-card">$$C = v \cdot G + r \cdot H$$</div>
        </div>
        <div>
          <div class="card"><strong>Poseidon Nullifier</strong></div>
          <div class="formula-card">$$N = \text{Poseidon}(SK_{\text{owner}}, \text{LeafIndex})$$</div>
        </div>
      </div>
      <div class="card" style="margin-top: 10px;"><strong>Groth16 Pairing Relation Verification</strong></div>
      <div class="formula-card">
        $$e(\pi_A, \pi_B) = e(\alpha, \beta) \cdot e\left( \gamma_0 + \sum_{i=1}^l x_i \gamma_i, \gamma \right) \cdot e(\pi_C, \delta)$$
      </div>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 8</span>
  </div>
</div>

<!-- Slide 9: Micro Benchmarks -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>Empirical Micro-benchmark Performance</h2>
      <p>Log-scale latency breakdown across core protocol engines</p>
    </div>
    <div class="slide-body fig-center">
      <img src="file:///FIG_DIR_PLACEHOLDER/micro_benchmarks.png" alt="Micro Benchmarks">
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 9</span>
  </div>
</div>

<!-- Slide 10: Consensus Scaling -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>HotStuff-2 BFT Consensus Scaling</h2>
      <p>2,015 ops/sec throughput under 100-validator supermajority</p>
    </div>
    <div class="slide-body fig-center">
      <img src="file:///FIG_DIR_PLACEHOLDER/consensus_scaling.png" alt="Consensus Scaling">
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 10</span>
  </div>
</div>

<!-- Slide 11: EIP-1559 Dynamics -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>EIP-1559 Dynamic Base Fee Response</h2>
      <p>Gas fee stability curve under fluctuating block utilization</p>
    </div>
    <div class="slide-body fig-center">
      <img src="file:///FIG_DIR_PLACEHOLDER/eip1559_fee_curve.png" alt="EIP-1559 Curve">
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 11</span>
  </div>
</div>

<!-- Slide 12: Conclusion -->
<div class="slide">
  <div>
    <div class="slide-header">
      <h2>Conclusion & Key Takeaways</h2>
      <p>Production-grade unified 3-layer modular blockchain</p>
    </div>
    <div class="slide-body">
      <ul>
        <li><strong>Unified Stack</strong>: Eliminates external layer dependencies and bridge exploit vectors.</li>
        <li><strong>Formally Verified</strong>: 34.1M+ TLA+ states verified with zero safety violations.</li>
        <li><strong>Developer & User Centric</strong>: Dual EVM+WASM runtimes, zero EOAs, multi-token gas fees.</li>
        <li><strong>High Scalability & Resilience</strong>: 2,015 ops/sec sustained under 100 validators, Jepsen fault resilient.</li>
      </ul>
      <div class="card" style="margin-top: 20px; text-align: center;">
        <strong>Project Website</strong>: https://viri.me &bull; <strong>Testnet RPC</strong>: https://rpc.viri.me
      </div>
    </div>
  </div>
  <div class="slide-footer">
    <span>Viri Research Presentation &bull; Purushottam Kumar</span>
    <span>Slide 12</span>
  </div>
</div>

</body>
</html>
"""

slides_html = slides_html.replace("FIG_DIR_PLACEHOLDER", fig_dir)

html_path = os.path.join(paper_dir, "viri_presentation_render.html")
with open(html_path, "w", encoding="utf-8") as f:
    f.write(slides_html)

print(f"Written presentation HTML to {html_path}")

pdf_path = os.path.join(paper_dir, "viri_presentation.pdf")

with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page()
    page.goto(f"file:///{html_path.replace('\\', '/')}")
    
    print("Waiting for MathJax in presentation...")
    page.wait_for_function("() => window.mathJaxDone === true")
    page.evaluate("() => MathJax.typesetPromise()")
    time.sleep(2)
    
    has_errors = page.evaluate("() => document.querySelectorAll('.mjx-merr').length")
    print(f"Presentation MathJax errors count: {has_errors}")
    
    print(f"Generating presentation PDF: {pdf_path}")
    page.pdf(
        path=pdf_path,
        width="16in",
        height="9in",
        print_background=True,
        margin={"top": "0", "bottom": "0", "left": "0", "right": "0"}
    )
    browser.close()

print(f"Successfully generated presentation PDF: {pdf_path}")
