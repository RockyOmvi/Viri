# Standalone Deployment

Deploy a node on any Linux server with Docker using the standalone provision script.

## Script

`deploy/scripts/standalone-provision.sh` is a cloud-agnostic Docker-only node provisioner.

### Usage

```bash
# Bootstrap node
sudo bash standalone-provision.sh bootstrap

# Validator node
sudo bash standalone-provision.sh validator \
  --index 0 \
  --bootstrap-addr <IP> \
  --bootstrap-peer-id <peer-id>

# Faucet node (with domain)
sudo bash standalone-provision.sh faucet \
  --faucet-key <hex-private-key> \
  --domain testnet.viri.me
```

## What It Does

1. Installs Docker if not present
2. Pulls the Viri Docker image
3. Creates the viri user and data directories
4. Configures the node based on role
5. Starts the node as a systemd service
6. Sets up log rotation

## Requirements

- Any Linux distribution with systemd
- Docker installed
- 1GB+ RAM
- 10GB+ disk space
