# Security Policy

## Responsible Disclosure

The Viri blockchain project welcomes security research and responsible disclosure of vulnerabilities.

## Reporting a Vulnerability

**Please do not file a public GitHub issue for security vulnerabilities.**
You can also create a [GitHub Security Advisory](https://github.com/viri-chain/viri/security/advisories/new) (preferred).

### What to include

- Description of the vulnerability
- Steps to reproduce (proof of concept preferred)
- Affected component(s) and version(s)
- Potential impact
- Suggested fix (if known)

### Response timeline

- **24 hours**: Acknowledgment of receipt
- **7 days**: Initial assessment with severity classification
- **30 days**: Fix released for critical/high severity issues
- **90 days**: Fix released for medium/low severity issues

## Scope

The following components are in scope:

- Consensus engine (`internal/layer1/consensus/`)
- P2P networking (`internal/layer1/p2p/`)
- Crypto implementations (`internal/layer1/crypto/`)
- VM runtimes (`internal/layer2/vm/`)
- Transaction execution (`internal/layer2/execution/`)
- ZK proof system (`internal/layer2/zk/`)
- Smart contract standards (`internal/layer2/contracts/`)
- Governance DAO (`internal/layer3/governance/`)
- Bridge (`internal/layer3/bridge/`)
- API server (`internal/layer3/api/`)
- CLI tools (`cmd/virictl/`, `cmd/virid/`)

Out of scope: third-party dependencies, infrastructure configurations.

## Bug Bounty

This project does not currently offer a bug bounty program. Security researchers are acknowledged in release notes for valid vulnerability reports.

## Disclosure Policy

1. Reporter discloses vulnerability privately
2. Project maintains confidentiality until fix is released
3. Fix is deployed with advisory notes
4. Reporter may disclose publicly after fix is released

## Supported Versions

| Version | Supported |
|---------|-----------|
| v0.1.x (testnet) | ✅ |
| main branch | ✅ |
| older versions | ❌ |

## Security Features

- Encrypted keystore with PBKDF2 key derivation
- BIP39 mnemonic backup
- Rate limiting (P2P + API)
- Peer reputation scoring with automatic ban
- Double-sign slashing
- View timeout and liveness detection
- Replay protection (timestamps, chain IDs)
- Fuzz testing (10 harnesses)
- Dependency vulnerability scanning (govulncheck)
- Static analysis (gosec, golangci-lint)
