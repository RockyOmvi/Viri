# Disaster Recovery Procedures

This document covers procedures for responding to severe network-wide incidents on the Viri mainnet. Each section follows a triage → diagnose → resolve → verify workflow.

## Severity Classification

| Severity | Definition | Response Time | Example |
|----------|------------|---------------|---------|
| **P1** | Chain halted or security breach | 15 minutes | Consensus stopped, key compromise |
| **P2** | Network degraded | 1 hour | High miss rate, slow block production |
| **P3** | Non-critical | 24 hours | Single node issue, slow peer discovery |

## Communication Channels

| Channel | Purpose | Access |
|---------|---------|--------|
| **Validator Signal Group** | Emergency coordination | All active validators |
| **Public Status Page** | Community updates | status.viri.network |
| **Discord #incidents** | Live incident discussion | Core devs + validators |
| **Email** | Formal post-mortems | security@viri.network |

---

## 1. Chain Halt

A chain halt occurs when the consensus engine stops producing blocks. The most common cause is loss of validator quorum (fewer than 2/3 of validators online).

### Detection

```bash
# Check block height (if it hasn't changed in >2 minutes, suspect halt)
curl -s http://localhost:8545/health | jq .height

# Check consensus state on validator nodes
curl -s -H "X-API-Key: $API_KEY" http://localhost:8545/ \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_getConsensusState","params":[],"id":1}'

# Prometheus alert fires: ViriChainStalled
# Expression: increase(viri_chain_block_height[10m]) == 0
```

### Diagnosis

```bash
# 1. Count online validators
for node in validator-01 validator-02 validator-03; do
  echo "$node: $(curl -s http://$node:8545/health | jq .status)"
done

# 2. Check peer connectivity
curl -s http://localhost:8546/api/v1/peers | jq '.peers | length'

# 3. Examine consensus logs
journalctl -u virid -n 200 --no-pager | grep -i "consensus\|timeout\|view_change\|error"

# 4. Check for network partition
# Run traceroute to each validator IP
traceroute -n <validator-ip>
```

### Recovery: Loss of Quorum

```bash
# 1. Contact offline validators via signal/discord/phone
#    They must restart their validator ASAP

# 2. If validator cannot be reached within 30 minutes (P1):
#    Proceed to fork recovery to replace the missing validator

# 3. While waiting, verify remaining validators have peer connectivity:
virictl peer list --rpc http://localhost:8545
```

### Recovery: Network Issue

```bash
# 1. Check if bootnodes are reachable
nc -zv <bootnode-ip> 30303

# 2. If NAT/firewall issue, verify port forwarding:
#    - Port 30303 TCP must be open on firewall
#    - Check external_addr in config matches public IP

# 3. Restart the node to re-establish connections
sudo systemctl restart virid

# 4. Monitor peer count recovery
watch -n 5 'curl -s http://localhost:8545/health | jq .peers'
```

---

## 2. Fork Recovery

A fork occurs when the network splits into two or more competing chains, typically due to a consensus bug, network partition, or software bug causing validators to disagree on block contents.

### Detection

```bash
# Compare block hashes at the same height across validators
curl -s http://validator-01:8545/ -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}' \
  | jq .result.hash

# Compare with:
curl -s http://validator-02:8545/ -X POST ...

# If hashes differ at the same height, a fork exists

# Log-based detection
journalctl -u virid -n 500 | grep -i "reorg\|fork\|orphan\|conflict"
```

### Recovery Procedure

**Step 1: Halt all validators**

The coordinator sends a stop signal to all validators via the emergency communication channel:

```bash
sudo systemctl stop virid
```

**Step 2: Determine the canonical chain**

The validators compare their latest block hashes. The canonical chain is determined by:

1. **Total accumulated stake** on each fork — the fork with more stake-weighted validators is canonical
2. **Block height** — if same stake weight, the longer chain wins
3. **Genesis hash** — all validators verify they are on the same genesis

**Step 3: Identify the fork point**

```bash
# Find where chains diverged (block N where hashes differ)
for height in $(seq $latest_block -1 0); do
  h1=$(curl -s http://val01:8545/ ... -d "{\"method\":\"eth_getBlockByNumber\",\"params\":[\"0x$(printf '%x' $height)\",false]}" | jq -r .result.hash)
  h2=$(curl -s http://val02:8545/ ... -d "{\"method\":\"eth_getBlockByNumber\",\"params\":[\"0x$(printf '%x' $height)\",false]}" | jq -r .result.hash)
  if [ "$h1" != "$h2" ]; then
    echo "Fork at block: $height"
    break
  fi
done
```

**Step 4: Validators on the minority fork roll back**

```bash
# Restore from backup taken before the fork point
sudo systemctl stop virid
virictl backup list --backup-dir /var/lib/viri/backups

# Find the most recent backup before the fork block
virictl backup restore /var/lib/viri/backups/viri-backup-<pre-fork>.tar.gz \
  --data-dir /var/lib/viri
sudo systemctl start virid
```

**Step 5: Resume consensus**

All validators restart after confirming they are on the same chain:

```bash
# All validators start simultaneously
sudo systemctl start virid

# Verify
watch -n 2 'curl -s http://localhost:8545/health | jq "{height: .height, peers: .peers, syncing: .syncing}"'
```

**Step 6: Post-recovery audit**

- Identify root cause of the fork (bug, configuration error, malicious validator)
- If bug: deploy patched software via emergency upgrade procedure
- If malicious validator: initiate slashing and removal via governance

---

## 3. State Corruption

State corruption occurs when the node's database becomes inconsistent, typically from disk errors, power loss, or software bugs.

### Detection

```bash
# Node crashes with DB errors
journalctl -u virid -n 100 | grep -i "corruption\|invalid\|checksum\|badger"

# Consensus invariant violations metric spikes
# Prometheus: consensus_invariant_violations_total

# Block execution fails on restart
```

### Recovery

```bash
# 1. Stop the node
sudo systemctl stop virid

# 2. Create backup of corrupted data (for forensic analysis)
sudo cp -r /var/lib/viri /var/lib/viri.corrupted.$(date +%Y%m%d%H%M%S)

# 3. Attempt automatic repair (restart may trigger repair)
sudo systemctl start virid

# 4. If still failing, restore from backup
sudo systemctl stop virid
virictl backup restore /var/lib/viri/backups/viri-backup-<latest>.tar.gz \
  --data-dir /var/lib/viri
sudo chown -R viri:viri /var/lib/viri
sudo systemctl start virid

# 5. If no backup available, use state sync from a healthy peer
# Delete data directory and restart to trigger state sync
sudo rm -rf /var/lib/viri/chaindata
sudo systemctl start virid
```

### Preventive Measures

- Enable database integrity checks (config: `storage.enable_integrity_checks: true`)
- Regular automated backups via `virid-backup.timer`
- Monitor disk health with SMART metrics
- Use enterprise-grade SSD with power-loss protection

---

## 4. Validator Key Compromise

If a validator's private key is exposed, the attacker can sign arbitrary blocks, double-sign, and cause the validator to be slashed.

### Detection

```bash
# Unexpected double-signing evidence in logs
journalctl -u virid | grep "double_sign\|equivocation\|duplicate_vote"

# Validator appearing to sign blocks they shouldn't
# Prometheus: consensus_invariant_violations_total
```

### Emergency Rotation Procedure

```bash
# 1. IMMEDIATELY: Stop the compromised validator
sudo systemctl stop virid

# 2. Generate new validator key on air-gapped machine
virictl wallet create
# Note the new address and public key

# 3. Transfer new key to the validator node (via encrypted USB)
sudo mkdir -p /etc/viri/keys
sudo cp new-validator.key.enc /etc/viri/keys/
sudo chown viri:viri /etc/viri/keys/new-validator.key.enc

# 4. Update config to use new key
# Edit node config: set node.validator_key to new key path

# 5. Submit on-chain validator key rotation transaction
# This requires governance approval (if governance module is active)
# OR manual update if governance bypass is needed (see Section 8)

# 6. Restart node with new key
sudo systemctl start virid

# 7. Revoke compromised key
# Publish a message on-chain signed by the new key
# declaring the old key compromised
```

### If the Compromised Key Was Used Maliciously

1. The network will detect double-signing and automatically slash the validator (config: `consensus.slashing_enabled: true`)
2. The slashed validator is removed from the active set
3. A new validator must apply to join via the normal process

### API Key Compromise

```bash
# 1. Generate new API key
NEW_KEY=$(openssl rand -hex 32)
NEW_HASH=$(echo -n "$NEW_KEY" | sha256sum --binary | xxd -p -c 64)

# 2. Update environment
sudo sed -i "s/VIRI_API_KEY_HASH=.*/VIRI_API_KEY_HASH=$NEW_HASH/" /etc/viri/virid.env

# 3. Restart
sudo systemctl restart virid

# 4. Distribute new key to authorized clients
echo "New API Key: $NEW_KEY"
```

---

## 5. Network Partition

A network partition splits validators into two or more groups that cannot communicate. After healing, the groups may be on different forks.

### Detection

```bash
# Peer count drops on all validators
# Prometheus: viri_p2p_peer_count near zero on affected nodes

# Consensus timeouts increase
# Prometheus: consensus_view_changes_total spikes

# Validators report "no peers" in health check
curl -s http://localhost:8545/health | jq .peers
```

### Recovery

```bash
# 1. Identify the partition boundary
# Which validators can talk to each other?

# 2. Check network infrastructure
# - Cloud provider status page for regional outages
# - ISP routing issues
# - Firewall rule changes

# 3. If partition was brief (<1 minute), validators will
#    automatically reconnect and continue producing blocks
#    on the longest chain via HotStuff BFT.

# 4. If partition persists (>1 minute) and forks occur:
#    a. Let the partition resolve naturally (ISP/DNS recovery)
#    b. Validators on each side will continue their own chain
#    c. When connectivity resumes, the canonical chain wins
#       (the side with more stake-weighted validators)
#    d. Validators on the losing fork must roll back:
sudo systemctl stop virid
virictl backup restore /var/lib/viri/backups/viri-backup-<pre-partition>.tar.gz
sudo systemctl start virid

# 5. If partition was caused by misconfiguration:
#    - Fix bootstrap_peers list
#    - Ensure external_addr is set correctly
#    - Verify firewall allows 30303/tcp from all validator IPs
```

### Healing After Split

After network connectivity is restored:

```bash
# 1. Monitor peer reconnection on each validator
watch -n 5 'curl -s http://localhost:8545/health | jq .peers'

# 2. Check that all validators converge on same chain
for val in validator-01 validator-02 validator-03; do
  echo "$val: $(curl -s http://$val:8545/health | jq '.height')"
done

# 3. If heights diverge, manually intervene (see Fork Recovery)
```

---

## 6. Data Loss

Complete or partial data loss on a validator node.

### Recovery from Peer State Sync

If the data directory is lost but the node key and validator key are preserved:

```bash
# 1. Stop the node
sudo systemctl stop virid

# 2. Delete only the chain data (keep keys and config)
sudo rm -rf /var/lib/viri/chaindata

# 3. Start the node - it will automatically sync from peers
sudo systemctl start virid

# 4. Monitor sync progress
watch -n 10 'curl -s http://localhost:8545/health | jq "{height: .height, syncing: .syncing}"'

# 5. Wait until syncing completes and node reaches the chain tip
```

### State Sync SLO Target

New or recovered nodes should sync to the chain tip within **1 hour**.

### Recovery from Backup

```bash
# 1. List available backups
virictl backup list --backup-dir /var/lib/viri/backups

# 2. Stop the node
sudo systemctl stop virid

# 3. Restore
virictl backup restore /var/lib/viri/backups/viri-backup-<latest>.tar.gz \
  --data-dir /var/lib/viri

# 4. Fix permissions
sudo chown -R viri:viri /var/lib/viri

# 5. Start
sudo systemctl start virid

# 6. Verify
curl -s http://localhost:8545/health | jq
```

---

## 7. 51% Attack Response

While HotStuff BFT provides tolerance up to 1/3 malicious validators, a cartel controlling >2/3 of stake can finalize invalid blocks.

### Detection

```bash
# Sudden, large reorganization
# Transactions being reverted that were thought finalized

# Abnormal block production
# Blocks produced faster than normal block time
# Blocks containing suspicious transactions

# Community reports of double-spends
# Users reporting funds lost
```

### Immediate Response

```bash
# 1. HALT THE CHAIN - ALL validators stop their nodes
# This prevents further damage
sudo systemctl stop virid

# 2. Preserve evidence
# Take snapshots of the current state
sudo cp -r /var/lib/viri /var/lib/viri.evidence.$(date +%Y%m%d%H%M%S)

# 3. Honest validators coordinate on a recovery fork
# See "Fork Recovery" section above

# 4. Identify the attacking validators
# Extract validator addresses from blocks at the fork height
```

### Recovery Fork (Post-51%)

```bash
# 1. Honest validators (>1/3 remaining stake) agree on a
#    recovery fork that EXCLUDES the attacking validators

# 2. Create a new genesis state at the last honest block
#    This is a "state-snapshot" fork

# 3. Remove attacking validators from the validator set
#    Update the consensus configuration

# 4. Distribute new genesis state to all honest nodes

# 5. Restart the network without attackers

# 6. Implement permanent solutions:
#    - Reduce finality threshold
#    - Increase validator bond requirements
#    - Add slashing conditions for governance attacks
```

### Long-Term Mitigations

- Implement **accountability** — cryptographic proofs of misbehavior that can be verified even after a 51% attack
- Use **checkpointing** to external chain (Ethereum, Bitcoin) for additional finality
- **Social consensus** — the community must be prepared to hard-fork to exclude malicious validators

---

## 8. Slashing Appeals Process

### Automatic Slashing

Viri implements slashing for:
- **Double signing** — signing two different blocks at the same height
- **Unavailability** — missing too many consecutive blocks (configurable)
- **Equivocation** — casting conflicting votes in the same consensus round

### Appeal Process

```bash
# 1. Validator receives slashing penalty
#    Node logs will contain the slashing evidence
journalctl -u virid | grep "slashing\|slash\|penalty"

# 2. Gather evidence proving innocence
#    - Logs showing node was operational
#    - Network connectivity proofs
#    - Timestamped audit logs

# 3. Submit appeal via governance proposal
#    Include: validator address, block height of infraction,
#    evidence bundle, explanation

# 4. Other validators vote on the appeal
#    - Simple majority (50%+1) to overturn
#    - If approved, slashed funds are restored
#    - If denied, slashing stands

# 5. Manual override (emergency only)
#    If governance is compromised, core devs can
#    initiate a genesis-restore to undo slashing
#    (only for proven false positives)
```

---

## 9. Emergency Upgrade Procedure

When the chain is halted or has a critical security vulnerability, standard governance procedures may be too slow. This procedure bypasses normal governance.

### Conditions for Emergency Upgrade

1. Chain is halted (no blocks for >10 minutes)
2. Active security vulnerability (funds at risk)
3. Critical consensus bug (network cannot produce valid blocks)

### Procedure

```bash
# 1. Core developers build a patched binary
go build -o virid-patched ./cmd/virid

# 2. Distribute the binary hash via emergency channel:
sha256sum virid-patched

# 3. All validators verify the binary hash independently
#    against the published hash from core devs

# 4. Validators stop their node:
sudo systemctl stop virid

# 5. Replace the binary:
sudo cp virid-patched /usr/local/bin/virid
sudo chmod +x /usr/local/bin/virid

# 6. If the upgrade changes consensus rules:
#    - The coordinator creates a new genesis or upgrade block
#    - All validators must agree on the upgrade point (block height)
#    - This is signaled by the block height in the config

# 7. All validators restart simultaneously:
sudo systemctl start virid

# 8. Monitor for the first blocks:
watch -n 5 'curl -s http://localhost:8545/health | jq "{height: .height, peers: .peers}"'

# 9. Once the network is stable (>100 blocks), normal
#    governance procedures resume
```

### Upgrade Block Signaling

For planned emergency upgrades, validators set the upgrade activation block in the node config:

```json
{
  "consensus": {
    "upgrade_block": 1000000,
    "upgrade_version": "0.2.0"
  }
}
```

At block 1,000,000, all nodes running the old binary will halt. Nodes running the new binary will continue.

---

## 10. Post-Incident Process

After every incident:

1. **Document timeline** — every action taken, by whom, at what time
2. **Root cause analysis** — determine why the incident occurred
3. **Update runbook** — add any new scenarios discovered
4. **Verify backups** — ensure all backup systems are operational
5. **Test recovery** — simulate the incident in a testnet environment
6. **Update monitoring** — add alerts for early detection of similar incidents
7. **Communicate** — publish a post-mortem to the community

## Emergency Contact List

| Role | Contact | Backup Contact |
|------|---------|----------------|
| Core Dev Lead | [Signal/Keybase] | [encrypted-email] |
| Validator Coordinator | [Signal/Keybase] | [phone] |
| Infrastructure Lead | [Signal/Keybase] | [phone] |
| Security Lead | [Signal/Keybase] | [encrypted-email] |

*Fill in actual contact details before mainnet launch.*
