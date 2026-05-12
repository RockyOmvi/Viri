#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Viri Public Testnet — One-Command Provision
# =============================================================================
# Prerequisites:
#   1. Oracle Cloud free tier account → 5 VMs (4 ARM + 1 AMD)
#   2. SSH key pair generated and added to each VM
#   3. Static public IPs assigned to each VM
#   4. Ports open in Oracle Cloud security list: 22, 30303, 8545, 8546, 8080, 8081, 9090
#   5. Ansible installed on your machine: pip install ansible
#
# Usage:
#   export BOOTSTRAP_PUBLIC_IP=<ip>
#   export VALIDATOR_0_IP=<ip>
#   export VALIDATOR_1_IP=<ip>
#   export VALIDATOR_2_IP=<ip>
#   export VALIDATOR_3_IP=<ip>
#   export FAUCET_PUBLIC_IP=<ip>
#   export DOMAIN=viri-testnet.io          # optional, for TLS
#   export SSH_KEY_PATH=~/.ssh/oracle_key  # optional, default ~/.ssh/id_rsa
#   ./one-command-provision.sh

echo "=== Viri Public Testnet Provisioner ==="

# --- Validate inputs ---
REQUIRED_VARS=(
  BOOTSTRAP_PUBLIC_IP VALIDATOR_0_IP VALIDATOR_1_IP
  VALIDATOR_2_IP VALIDATOR_3_IP FAUCET_PUBLIC_IP
)
for var in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!var:-}" ]; then
    echo "ERROR: $var is not set"
    exit 1
  fi
done

SSH_KEY_PATH="${SSH_KEY_PATH:-$HOME/.ssh/id_rsa}"
DOMAIN="${DOMAIN:-}"

echo "  Bootstrap:  $BOOTSTRAP_PUBLIC_IP"
echo "  Validator-0: $VALIDATOR_0_IP"
echo "  Validator-1: $VALIDATOR_1_IP"
echo "  Validator-2: $VALIDATOR_2_IP"
echo "  Validator-3: $VALIDATOR_3_IP"
echo "  Faucet:     $FAUCET_PUBLIC_IP"
echo "  SSH Key:    $SSH_KEY_PATH"
echo "  Domain:     ${DOMAIN:-<none>}"

# --- Step 1: Bootstrap node generates its key and peer ID ---
echo ""
echo "=== Step 1: Setting up bootstrap node ==="

ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no "ubuntu@$BOOTSTRAP_PUBLIC_IP" <<'REMOTE'
  sudo mkdir -p /opt/viri/config /opt/viri/data /opt/viri/logs
  if [ ! -f /opt/viri/config/bootstrap.key ]; then
    docker pull ghcr.io/viri-chain/viri:latest
    docker run --rm -v /opt/viri/config:/config ghcr.io/viri-chain/viri:latest \
      virid --generate-node-key --key-file /config/bootstrap.key
  fi
REMOTE

# Extract peer ID from bootstrap node
BOOTSTRAP_PEER_ID=$(ssh -i "$SSH_KEY_PATH" "ubuntu@$BOOTSTRAP_PUBLIC_IP" \
  "sudo cat /opt/viri/config/bootstrap.key | grep -oP 'peer_id:\s*\K\S+'" 2>/dev/null || echo "")

if [ -z "$BOOTSTRAP_PEER_ID" ]; then
  echo "ERROR: Could not read bootstrap peer ID"
  exit 1
fi
echo "  Bootstrap Peer ID: $BOOTSTRAP_PEER_ID"

# --- Step 2: Deploy via Ansible ---
echo ""
echo "=== Step 2: Running Ansible playbook ==="

cd "$(dirname "$0")/ansible"

# Write inventory file with actual IPs
cat > inventory/oracle-cloud.yml <<INVENTORY
all:
  children:
    bootstrap:
      hosts:
        bootstrap-node:
          ansible_host: $BOOTSTRAP_PUBLIC_IP
    validators:
      hosts:
        validator-0:
          ansible_host: $VALIDATOR_0_IP
          validator_id: 0
        validator-1:
          ansible_host: $VALIDATOR_1_IP
          validator_id: 1
        validator-2:
          ansible_host: $VALIDATOR_2_IP
          validator_id: 2
        validator-3:
          ansible_host: $VALIDATOR_3_IP
          validator_id: 3
    faucet:
      hosts:
        faucet-node:
          ansible_host: $FAUCET_PUBLIC_IP
  vars:
    ansible_user: ubuntu
    ansible_ssh_private_key_file: $SSH_KEY_PATH
INVENTORY

# Run the playbook
ANSIBLE_HOST_KEY_CHECKING=False \
ansible-playbook \
  -i inventory/oracle-cloud.yml \
  deploy-testnet.yml \
  --extra-vars "bootstrap_peer_id=$BOOTSTRAP_PEER_ID" \
  --extra-vars "BOOTSTRAP_PUBLIC_IP=$BOOTSTRAP_PUBLIC_IP" \
  --extra-vars "VALIDATOR_0_IP=$VALIDATOR_0_IP" \
  --extra-vars "VALIDATOR_1_IP=$VALIDATOR_1_IP" \
  --extra-vars "VALIDATOR_2_IP=$VALIDATOR_2_IP" \
  --extra-vars "VALIDATOR_3_IP=$VALIDATOR_3_IP" \
  --extra-vars "FAUCET_PUBLIC_IP=$FAUCET_PUBLIC_IP" \
  ${DOMAIN:+--extra-vars "domain=$DOMAIN"}

# --- Step 3: Verify ---
echo ""
echo "=== Step 3: Verification ==="

echo ""
echo "All nodes deployed! Here are your endpoints:"
echo "  Explorer:  http://$FAUCET_PUBLIC_IP:8080"
echo "  Faucet:    http://$FAUCET_PUBLIC_IP:8081"
echo "  RPC:       http://$BOOTSTRAP_PUBLIC_IP:8545"
echo "  API:       http://$BOOTSTRAP_PUBLIC_IP:8546"
echo ""
echo "To check node status:"
echo "  curl http://$BOOTSTRAP_PUBLIC_IP:8545/health"
echo "  curl http://$BOOTSTRAP_PUBLIC_IP:8546/admin/status"
echo ""
echo "To watch logs:"
echo "  ssh -i $SSH_KEY_PATH ubuntu@$BOOTSTRAP_PUBLIC_IP 'docker logs -f virid-bootstrap'"
