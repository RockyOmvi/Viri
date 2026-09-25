import matplotlib.pyplot as plt
import numpy as np
import os

# Ensure paper/figures directory exists
fig_dir = os.path.join(os.path.dirname(__file__), "figures")
os.makedirs(fig_dir, exist_ok=True)

# Set publication style
plt.style.use('seaborn-v0_8-paper' if 'seaborn-v0_8-paper' in plt.style.available else 'default')
plt.rcParams.update({
    'font.sans-serif': 'DejaVu Sans',
    'font.family': 'sans-serif',
    'figure.autolayout': True,
    'font.size': 11,
    'axes.labelsize': 12,
    'axes.titlesize': 13,
    'xtick.labelsize': 10,
    'ytick.labelsize': 10,
    'legend.fontsize': 10,
    'figure.titlesize': 14
})

# -------------------------------------------------------------
# Chart 1: Consensus Scalability & Finality Latency
# -------------------------------------------------------------
validators = ['4 Nodes', '16 Nodes', '50 Nodes', '100 Nodes']
val_x = np.arange(len(validators))
ops_sec = [12450, 8200, 4100, 2015]
finality_ms = [510, 680, 920, 1450]

fig, ax1 = plt.subplots(figsize=(6.5, 4.2))

color = '#1f77b4'
ax1.set_xlabel('Validator Network Size')
ax1.set_ylabel('Consensus Throughput (Ops / sec)', color=color, fontweight='bold')
bars = ax1.bar(val_x - 0.15, ops_sec, width=0.3, color=color, label='Ops / sec', alpha=0.85, edgecolor='black')
ax1.tick_params(axis='y', labelcolor=color)
ax1.set_ylim(0, 14000)

# Add data values on bars
for bar in bars:
    yval = bar.get_height()
    ax1.text(bar.get_x() + bar.get_width()/2.0, yval + 200, f'{yval:,}', ha='center', va='bottom', fontsize=9, color=color, fontweight='bold')

ax2 = ax1.twinx()
color = '#d62728'
ax2.set_ylabel('Block Finality Latency (ms)', color=color, fontweight='bold')
line = ax2.plot(val_x + 0.15, finality_ms, color=color, marker='o', linewidth=2.5, markersize=8, label='Finality Latency (ms)')
ax2.tick_params(axis='y', labelcolor=color)
ax2.set_ylim(0, 1800)

for i, txt in enumerate(finality_ms):
    ax2.annotate(f'{txt} ms', (val_x[i] + 0.15, finality_ms[i] + 40), ha='center', color=color, fontweight='bold', fontsize=9)

plt.title('HotStuff-2 Consensus Scaling & Finality Latency', fontweight='bold', pad=12)
ax1.set_xticks(val_x)
ax1.set_xticklabels(validators)
ax1.grid(True, linestyle='--', alpha=0.4)

plt.savefig(os.path.join(fig_dir, 'consensus_scaling.png'), dpi=300)
plt.close()
print("Saved consensus_scaling.png")

# -------------------------------------------------------------
# Chart 2: Micro-benchmark Latency Breakdown (Log Scale)
# -------------------------------------------------------------
operations = [
    'Consensus Step', 'MEV Batch', 'Rate Limiter', 'P2P Encode',
    'Account Transfer', 'Submit Batch', 'State Acc Create',
    'Deploy Contract', 'ECDSA Sign', 'Tx Pool Insert',
    'ECDSA Verify', 'Add Block', 'Merkle Tree (1k)'
]

latencies_ns = [
    11, 11, 34, 37,
    101, 103, 214,
    1400, 28000, 115000,
    123000, 241000, 337000
]

fig, ax = plt.subplots(figsize=(8, 5))
y_pos = np.arange(len(operations))

colors = plt.cm.plasma(np.linspace(0.2, 0.85, len(operations)))
bars = ax.barh(y_pos, latencies_ns, color=colors, edgecolor='black', alpha=0.9)

ax.set_xscale('log')
ax.set_yticks(y_pos)
ax.set_yticklabels(operations, fontweight='bold')
ax.invert_yaxis()  # top-down
ax.set_xlabel('Execution Latency per Operation (Nanoseconds, Log Scale)', fontweight='bold')
ax.set_title('Viri Protocol Engine Micro-benchmark Latencies', fontweight='bold', pad=12)
ax.grid(True, which="both", linestyle="--", alpha=0.3)

# Add exact text tags
for bar, lat in zip(bars, latencies_ns):
    width = bar.get_width()
    if lat >= 1000000:
        label_text = f"{lat/1000000:.2f} ms"
    elif lat >= 1000:
        label_text = f"{lat/1000:.1f} µs"
    else:
        label_text = f"{lat} ns"
    ax.text(width * 1.2, bar.get_y() + bar.get_height()/2.0, label_text, ha='left', va='center', fontsize=9, fontweight='bold')

ax.set_xlim(1, 10000000)

plt.savefig(os.path.join(fig_dir, 'micro_benchmarks.png'), dpi=300)
plt.close()
print("Saved micro_benchmarks.png")

# -------------------------------------------------------------
# Chart 3: EIP-1559 Dynamic Base Fee Response Curve
# -------------------------------------------------------------
blocks = np.arange(1, 51)
target_gas = 15_000_000
limit_gas = 30_000_000

# Simulate gas utilization wave
np.random.seed(42)
gas_used = target_gas + (target_gas * 0.8 * np.sin(blocks / 4.0)) + np.random.normal(0, target_gas * 0.1, len(blocks))
gas_used = np.clip(gas_used, 0, limit_gas)

base_fees = [100.0]  # Initial base fee in Gwei
d = 0.125
for g in gas_used[:-1]:
    prev_fee = base_fees[-1]
    adj = 1.0 + d * ((g - target_gas) / target_gas)
    new_fee = max(10.0, prev_fee * adj)
    base_fees.append(new_fee)

fig, ax1 = plt.subplots(figsize=(7, 4.2))

color = '#2ca02c'
ax1.set_xlabel('Block Height (n)')
ax1.set_ylabel('Block Gas Utilization (M Gas)', color=color, fontweight='bold')
ax1.plot(blocks, gas_used / 1e6, color=color, linestyle='-', linewidth=2, label='Gas Used (M)')
ax1.axhline(target_gas / 1e6, color='gray', linestyle=':', label='Target Gas (15M)')
ax1.tick_params(axis='y', labelcolor=color)
ax1.set_ylim(0, 32)

ax2 = ax1.twinx()
color = '#ff7f0e'
ax2.set_ylabel('EIP-1559 Base Fee (Gwei)', color=color, fontweight='bold')
ax2.plot(blocks, base_fees, color=color, linestyle='--', linewidth=2.5, label='Base Fee (Gwei)')
ax2.tick_params(axis='y', labelcolor=color)

plt.title('Dynamic EIP-1559 Base Fee Response under Gas Demand Elasticity', fontweight='bold', pad=12)
ax1.grid(True, linestyle='--', alpha=0.3)

plt.savefig(os.path.join(fig_dir, 'eip1559_fee_curve.png'), dpi=300)
plt.close()
print("Saved eip1559_fee_curve.png")
