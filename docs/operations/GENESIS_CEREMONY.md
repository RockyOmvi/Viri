# Genesis Ceremony Procedure

The genesis ceremony bootstraps the Viri mainnet validator set and produces the canonical `genesis.json` that every node must use to join the network. The ceremony uses `virictl genesis` subcommands in a multi-step, multi-party process with cryptographic verification at each stage.

## Roles

| Role | Responsibility |
|------|----------------|
| **Coordinator** | Initializes ceremony directory, collects contributions, finalizes genesis |
| **Validator** | Generates key, registers public key, signs manifest |
| **Witness** | Cross-checks signatures, verifies final genesis hash |

## Step-by-Step Procedure

### Phase 1: Pre-Ceremony Preparation

Each validator generates their key material before the ceremony begins.

```bash
# Generate a BIP39 mnemonic (12 words) for the validator key
virictl mnemonic generate 12 > validator-mnemonic.txt

# Derive the public key and address
virictl mnemonic to-key "$(cat validator-mnemonic.txt)"

# Output:
# Address:    0xabc123...
# Public Key: 0x04def456...
```

The mnemonic must be stored securely offline (paper backup, hardware wallet, or encrypted USB). The public key and address are shared with the coordinator.

**Important:** Key generation happens on an **air-gapped machine** that has never been connected to the internet.

### Phase 2: Coordinator Initialization

The coordinator creates the ceremony directory on a secure, well-connected machine:

```bash
virictl genesis init \
  --dir .viri/genesis \
  --chain-id 1 \
  --network viri-mainnet
```

This creates:
- `ceremony.json` — chain parameters (chain_id, network_name, genesis_time, min_validators, initial_supply)
- `manifest.json` — initially empty validator list, set to `complete: false`
- `state.json` — ceremony phase tracking (`phase: "init"`)
- `ceremony_log.json` — signed audit trail of every ceremony action

### Phase 3: Validator Registration

Each validator submits their public key to the coordinator via a secure out-of-band channel (encrypted email, Keybase, or in-person meeting).

The coordinator adds each validator:

```bash
# Format: virictl genesis add-validator --name <name> --pubkey <hex> --stake <amount>
virictl genesis add-validator \
  --dir .viri/genesis \
  --name "Validator-Alice" \
  --pubkey "04abc123..." \
  --stake 10000000

virictl genesis add-validator \
  --dir .viri/genesis \
  --name "Validator-Bob" \
  --pubkey "04def456..." \
  --stake 10000000

virictl genesis add-validator \
  --dir .viri/genesis \
  --name "Validator-Charlie" \
  --pubkey "04ghi789..." \
  --stake 10000000

# Continue for all validators...
```

After all validators are registered, verify:

```bash
virictl genesis status --dir .viri/genesis
```

The ceremony phase transitions to `"registration"` after the first validator is added.

### Phase 4: Manifest Distribution

The coordinator exports the signing payload and distributes it to all validators:

```bash
virictl genesis export-payload --dir .viri/genesis
```

This creates `signing_payload.json` containing:
- `manifest_hash` — SHA-256 hash of the manifest (all validator entries + chain config)
- `validators` — complete list of registered validators
- `genesis_time`, `chain_id`, `network_name`

The signing payload is transferred to each validator's **offline signing machine**.

### Phase 5: Validator Signing (Offline)

Each validator, on their air-gapped machine:

```bash
# Load the signing payload received from coordinator
# Decrypt your validator key and sign the manifest

virictl genesis sign \
  --dir .viri/genesis \
  --key /path/to/encrypted-validator-key.key \
  --passphrase "<key-passphrase>"
```

If passphrase is omitted, the tool prompts interactively. The command:
1. Decrypts the keystore file using the passphrase
2. Computes `manifest_hash` from the manifest
3. Signs the hash with the validator's secp256k1 private key
4. Saves the signature to `sig_<address>.json`

The output signature file contains:
```json
{
  "validator_address": "0xabc123...",
  "public_key": "04abc123...",
  "signature": "3045022100...",
  "timestamp": "2026-06-01T12:00:00Z",
  "manifest_hash": "a1b2c3d4..."
}
```

The signature file is transferred back to the coordinator.

### Phase 6: Signature Import

The coordinator imports each validator's signature:

```bash
virictl genesis import-signature \
  --dir .viri/genesis \
  --file sig_0xabc123.json

virictl genesis import-signature \
  --dir .viri/genesis \
  --file sig_0xdef456.json
```

### Phase 7: Verification (Social Ceremony)

All validators participate in verifying the collected signatures:

```bash
virictl genesis verify --dir .viri/genesis
```

This performs:
1. **Signature count check** — at least 2/3+1 of validators must have signed
2. **Stake-weighted quorum check** — signed stake must exceed 2/3+1 of total stake
3. **Cryptographic verification** — each signature is verified against the manifest hash
4. **Validator identity check** — each signer must be a registered validator
5. **Manifest hash consistency** — all signatures must reference the same manifest hash

Example output:
```
Validators: 7
Total stake: 70000000
Quorum required (validators): 5
Quorum required (stake): 46666667
Signatures collected: 6

  [VALID]   0xabc123... - signed at 2026-06-01T12:00:00Z
  [VALID]   0xdef456... - signed at 2026-06-01T12:05:00Z
  [INVALID] 0xbadbad... - signature verification failed
  ...

Unsigned validators:
  - Validator-Dave (0xeee...) - stake: 10000000
```

**Cross-Verification Ceremony:**

Each validator independently:
1. Obtains the final `manifest.json` from the coordinator
2. Downloads all `sig_*.json` files from a public repository or the coordinator
3. Runs `virictl genesis verify --dir .viri/genesis` on their own machine
4. Confirms the output matches in a group call or shared document

The genesis hash is the **canonical identifier** of the chain. Every validator **must** arrive at the same hash.

### Phase 8: Finalization

Once quorum is reached and all cross-checks pass:

```bash
# From the coordinator
virictl genesis finalize --dir .viri/genesis
```

This produces `genesis.json` with:
```json
{
  "chain_id": 1,
  "network_name": "viri-mainnet",
  "genesis_time": "2026-06-01T14:00:00Z",
  "genesis_hash": "a1b2c3d4e5...",
  "validators": [...],
  "total_stake": 70000000,
  "version": "0.1.0"
}
```

The final `genesis_hash` **must** be distributed to all participants and published on public channels (website, GitHub, Twitter). Every node operator verifies this hash before starting their node.

### Phase 9: Genesis Hash Verification

Every participant independently verifies the genesis hash:

```bash
# 1. Download genesis.json from coordinator
# 2. Compute hash locally
sha256sum genesis.json

# 3. Compare with published hash
echo "a1b2c3d4e5...  genesis.json" | sha256sum -c -

# 4. Verify validator set matches
virictl genesis verify --dir .viri/genesis
```

All validators confirm in a group communication that their computed hash matches.

### Phase 10: Genesis Day

The coordinator publishes the final `genesis.json` to:
- GitHub releases page
- A public, immutable file store (IPFS, Arweave)
- Official website download link
- All validators via secure channel

Each validator places the genesis file and starts their node:

```bash
# Place genesis file
cp genesis.json configs/genesis/mainnet.json

# Start the node
virid --config configs/node-mainnet.json
```

## Handling a Validator Going Offline

If a validator becomes unreachable during the ceremony:

1. **During registration (Phase 3):** The coordinator sets a registration deadline. Missing validators are excluded from the manifest.

2. **During signing (Phase 5):**
   - If quorum is already met without them, proceed to finalization
   - If quorum is not met, extend the signing window (e.g., +24 hours)
   - After the extended deadline, exclude the missing validator and re-distribute the updated manifest
   - All validators who already signed must re-sign the new manifest

3. **After finalization:** The validator cannot join until the next validator set rotation (via on-chain governance after genesis).

### Procedure for Validator Exclusion

```bash
# 1. Coordinator creates a new manifest excluding the offline validator
# Move the current manifest aside
cp manifest.json manifest.json.bak

# 2. Remove offline validator entries and re-distribute
# All existing signers must now sign the NEW manifest

# 3. Previous signatures are invalidated (different manifest hash)
```

**Important:** A new manifest means a **different genesis hash**. Every validator must re-verify and confirm the new hash.

## Checklist for Genesis Day

### T-Minus 7 Days
- [ ] All validators generate their keys on air-gapped machines
- [ ] Public keys shared with coordinator
- [ ] Ceremony directory initialized
- [ ] Network parameters finalized (chain_id, block_time, initial_supply, max_validators)

### T-Minus 3 Days
- [ ] Validators registered in manifest
- [ ] Signing payload exported and distributed
- [ ] Validators begin signing on offline machines

### T-Minus 1 Day
- [ ] All signatures collected and imported
- [ ] Verification ceremony conducted
- [ ] Cross-checks completed by all validators
- [ ] Genesis hash computed and distributed

### T-Minus 6 Hours
- [ ] Final genesis.json assembled
- [ ] Genesis hash published to public channels
- [ ] Seed/bootnodes configured and ready

### T-Minus 1 Hour
- [ ] All validators confirm genesis hash matches
- [ ] Node software updated to release version
- [ ] Infrastructure readiness verified (monitoring, backups, firewalls)

### Genesis Time (T-0)
- [ ] Coordinator signal sent: "GENESIS START"
- [ ] All validators start their nodes simultaneously
- [ ] Block 0 produced

### Post-Genesis (+15 minutes)
- [ ] Block 1+ produced (verify block time)
- [ ] All validators producing blocks
- [ ] Peer connections stable
- [ ] Monitoring alerts configured and verified
- [ ] Genesis ceremony artifacts archived (ceremony_log.json, all sig files, genesis.json)

## Ceremony Audit Trail

Every action in the genesis ceremony is logged to `ceremony_log.json`:

```json
[
  {
    "timestamp": "2026-06-01T10:00:00Z",
    "event_type": "ceremony initialized",
    "details": "chain_id=1 network=viri-mainnet"
  },
  {
    "timestamp": "2026-06-01T10:05:00Z",
    "event_type": "validator registered",
    "details": "name=Validator-Alice address=0xabc... stake=10000000"
  },
  {
    "timestamp": "2026-06-01T12:00:00Z",
    "event_type": "validator signed",
    "details": "address=0xabc...",
    "signature": "304502..."
  },
  {
    "timestamp": "2026-06-01T14:00:00Z",
    "event_type": "ceremony finalized",
    "details": "hash=a1b2c3... validators=7"
  }
]
```

## Security Considerations

- **Never** transmit private keys over any network
- **Never** perform key generation on a machine that has been or will be connected to the internet
- **Physically verify** validator identities before adding to manifest
- The ceremony directory contains sensitive intermediate files; access must be restricted to the coordinator
- The final genesis.json and ceremony_log.json are public; all other files (sig_*.json, manifest.json) can be published for transparency
- If any validator detects tampering, the ceremony must be aborted and restarted from the last trusted state
