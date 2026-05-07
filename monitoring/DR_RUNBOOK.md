# Viri Node Disaster Recovery Runbook

## 1. Node Won't Start

### Symptoms
- `systemctl status virid` shows failed state
- Logs show panic or fatal error

### Diagnosis
```bash
# Check recent logs
journalctl -u virid -n 100 --no-pager

# Check disk space
df -h /var/lib/viri

# Check file permissions
ls -la /var/lib/viri/
```

### Resolution Steps

#### A. Corrupt Database
```bash
# 1. Stop the node
sudo systemctl stop virid

# 2. Backup current data
sudo cp -r /var/lib/viri /var/lib/viri.backup.$(date +%Y%m%d)

# 3. Try BadgerDB repair (if using Badger)
# The node will attempt automatic repair on restart

# 4. Restart
sudo systemctl start virid

# 5. If still failing, restore from backup
sudo rm -rf /var/lib/viri
sudo cp -r /var/lib/viri.backup.YYYYMMDD /var/lib/viri
sudo systemctl start virid
```

#### B. Missing TLS Certificates
```bash
# Check cert existence
ls -la /etc/viri/tls/

# Regenerate self-signed certs (temporary fix)
sudo openssl req -x509 -nodes -days 7 \
  -newkey rsa:2048 \
  -keyout /etc/viri/tls/server.key \
  -out /etc/viri/tls/server.crt \
  -subj "/CN=viri-node"
sudo chmod 640 /etc/viri/tls/server.key
sudo chown viri:viri /etc/viri/tls/server.*

sudo systemctl start virid
```

#### C. Permission Issues
```bash
sudo chown -R viri:viri /var/lib/viri /etc/viri
sudo chmod 750 /var/lib/viri
sudo chmod 600 /etc/viri/virid.env
sudo systemctl start virid
```

---

## 2. Chain Stall (No New Blocks)

### Symptoms
- Block height hasn't increased in >5 minutes
- `GET /ready` returns 503
- Prometheus alert `ViriChainStalled` fired

### Diagnosis
```bash
# Check current height
curl -s http://127.0.0.1:8545/health | jq

# Check peer count
curl -s http://127.0.0.1:8545/health | jq .peers

# Check consensus status (validator nodes)
curl -s -H "X-API-Key: $API_KEY" https://localhost:8545/ \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"viri_getConsensusState","params":[],"id":1}'

# Check logs for consensus errors
journalctl -u virid -n 200 --no-pager | grep -i "consensus\|error\|fail"
```

### Resolution Steps

#### A. Lost Peers
```bash
# 1. Check network connectivity
curl -s http://127.0.0.1:8546/api/v1/peers | jq

# 2. Restart network layer
sudo systemctl restart virid

# 3. If using bootnodes, verify they're reachable
# Add known good peers manually via P2P protocol
```

#### B. Validator Quorum Lost
```bash
# 1. Check if enough validators are online
# Minimum 2/3+1 validators must be active for HotStuff

# 2. Contact offline validators via out-of-band channel

# 3. If permanent validator loss:
#    a. Stop all validators
#    b. Remove inactive validator from config
#    c. Restart with updated validator set
```

#### C. Database Corruption
Follow "Corrupt Database" steps above, then restore from latest backup.

---

## 3. Chain Reorg / Fork Recovery

### Symptoms
- Multiple competing chains detected
- Block height decreased (reorg)
- Transactions missing after reorg

### Diagnosis
```bash
# Validate chain continuity
# (Requires custom RPC endpoint or direct DB check)

# Check for orphaned blocks
journalctl -u virid -n 500 | grep -i "reorg\|fork\|orphan"
```

### Resolution Steps

```bash
# 1. Stop the node
sudo systemctl stop virid

# 2. Export current state snapshot (if possible)
# virictl backup create --data-dir /var/lib/viri

# 3. Restore from pre-fork backup
sudo systemctl stop virid
virictl backup restore /var/lib/viri/backups/viri-backup-YYYYMMDD-HHMMSS.tar.gz
sudo systemctl start virid

# 4. Wait for sync to catch up
watch -n 5 'curl -s http://127.0.0.1:8545/health | jq .height'

# 5. Verify chain continuity
# Once synced, check that block hashes match other validators
```

---

## 4. Emergency Shutdown

### When to Use
- Security breach detected
- Critical bug discovered
- Regulatory requirement

### Steps
```bash
# 1. Immediate stop
sudo systemctl stop virid

# 2. Create emergency backup
sudo cp -r /var/lib/viri /var/lib/viri.emergency.$(date +%Y%m%d%H%M%S)

# 3. Save mempool state (if node was running)
# (Already handled by graceful shutdown)

# 4. Block network access
sudo ufw deny 30303/tcp
sudo ufw deny 8545/tcp
sudo ufw deny 8546/tcp

# 5. Notify validators via out-of-band channel

# 6. Document incident
# Record timestamp, reason, and actions taken
```

### Restart After Emergency
```bash
# 1. Review and patch the issue
# 2. Restore from emergency backup if needed
# 3. Re-enable network access
sudo ufw allow 30303/tcp

# 4. Start node
sudo systemctl start virid

# 5. Verify sync and consensus
curl -s http://127.0.0.1:8545/ready | jq

# 6. Monitor for 30 minutes before declaring resolved
```

---

## 5. Backup Recovery

### When to Use
- Database corruption
- Accidental data deletion
- Migration to new hardware

### Steps
```bash
# 1. List available backups
virictl backup list --backup-dir /var/lib/viri/backups

# 2. Stop the node
sudo systemctl stop virid

# 3. Restore backup
virictl backup restore /var/lib/viri/backups/viri-backup-YYYYMMDD-HHMMSS.tar.gz \
  --data-dir /var/lib/viri

# 4. Fix permissions
sudo chown -R viri:viri /var/lib/viri

# 5. Start node
sudo systemctl start virid

# 6. Verify recovery
curl -s http://127.0.0.1:8545/health | jq
curl -s http://127.0.0.1:8545/ready | jq
```

---

## 6. Key Compromise Recovery

### When to Use
- Validator private key leaked
- API key exposed
- Keystore passphrase compromised

### Steps

#### A. Validator Key Compromise
```bash
# 1. Emergency shutdown (see Section 4)

# 2. Generate new validator key
export VIRI_KEY_PASSPHRASE="new-strong-passphrase"
# Node will generate new key on restart

# 3. Update validator set
# Remove compromised validator via governance
# Add new validator with updated key

# 4. Restart with new key
sudo systemctl start virid
```

#### B. API Key Compromise
```bash
# 1. Generate new API key
NEW_KEY=$(openssl rand -hex 32)
NEW_HASH=$(echo -n "$NEW_KEY" | sha256sum --binary | xxd -p -c 64)

# 2. Update environment
sudo sed -i "s/VIRI_API_KEY_HASH=.*/VIRI_API_KEY_HASH=$NEW_HASH/" /etc/viri/virid.env

# 3. Restart to apply
sudo systemctl restart virid

# 4. Distribute new key to authorized clients
echo "New API Key: $NEW_KEY"
```

---

## 7. Disk Space Emergency

### Symptoms
- "no space left on device" errors
- Database writes failing
- Node crashing

### Steps
```bash
# 1. Check disk usage
df -h /var/lib/viri
du -sh /var/lib/viri/*

# 2. Clean old backups (keep last 3)
cd /var/lib/viri/backups
ls -lt | tail -n +4 | awk '{print $NF}' | xargs rm -f

# 3. Clean old audit logs
cd /var/lib/viri/logs
ls -lt | tail -n +4 | awk '{print $NF}' | xargs rm -f

# 4. Compact BadgerDB (if applicable)
# This requires stopping the node first
sudo systemctl stop virid
# Badger auto-compacts on restart

# 5. Restart node
sudo systemctl start virid

# 6. Long-term: Add disk or enable pruning
# Update config: Storage.PruningEnabled = true
```

---

## Contact Escalation

| Severity | Response Time | Action |
|----------|---------------|--------|
| P1 - Chain halted | 15 min | Emergency shutdown, notify all validators |
| P2 - Degraded | 1 hour | Diagnose, apply fix, monitor |
| P3 - Non-critical | 24 hours | Schedule maintenance window |

### On-Call Contacts
- Validator operators: [Out-of-band channel]
- Core devs: [Contact list]
- Infrastructure: [Contact list]

---

## Post-Incident Checklist

After any incident:
- [ ] Document timeline and root cause
- [ ] Update runbook if new scenario discovered
- [ ] Verify all backups are current
- [ ] Test restore procedure on staging
- [ ] Update monitoring alerts if gaps found
- [ ] Communicate status to stakeholders
