# Viri Public Testnet — Deployment Guide

## Prerequisites

- **Azure subscription** with $100 credit
- **Azure CLI** — [install](https://aka.ms/installazurecliwindows)
- **Terraform** — [install](https://developer.hashicorp.com/terraform/install)
- **SSH key pair** — `ssh-keygen -t ed25519 -f ~/.ssh/viri-azure -C "viri-testnet"`
- **Docker image** built and pushed to `ghcr.io/viri-chain/viri:latest`

## Step 1: Authenticate with Azure

```bash
az login
az account set --subscription "<subscription-id>"
```

## Step 2: Configure Terraform

```bash
cd deploy/terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:
- Set `location` to your closest Azure region (e.g., `eastus`, `westeurope`, `southeastasia`)
- Set `ssh_public_key` to your public key content
- Set `vm_sku` to `Standard_B1ms` for cost savings (~$91/month, fits $100 credit for ~33 days)
- Optionally set `domain` and `faucet_wallet_key`

## Step 3: Deploy Infrastructure

```bash
terraform init
terraform plan
terraform apply -auto-approve
```

This provisions 7 VMs:
- `viri-bootstrap` — P2P bootstrap node + RPC endpoint
- `viri-validator-[0-3]` — 4 block-producing validators
- `viri-faucet` — Faucet + Block Explorer
- `viri-monitoring` — Prometheus + Grafana + Alertmanager

After completion, Terraform outputs all public IPs:
```
instance_ips = {
  "bootstrap"    = "20.x.x.1"
  "validator-0"  = "20.x.x.2"
  "validator-1"  = "20.x.x.3"
  "validator-2"  = "20.x.x.4"
  "validator-3"  = "20.x.x.5"
  "faucet"       = "20.x.x.6"
  "monitoring"   = "20.x.x.7"
}
```

## Step 4: Extract Bootstrap Peer ID

```bash
BOOTSTRAP_IP=$(terraform output -raw bootstrap_ip)
ssh -i ~/.ssh/viri-azure viriadmin@$BOOTSTRAP_IP "cat /opt/viri/config/bootstrap-id.json"
```

This returns the peer ID (e.g., `{"peer_id":"12D3KooW..."}`). Save this value.

## Step 5: Fill Bootstrap IPs in Node Configs

Update `configs/node-testnet.json` with the real bootstrap IP and peer ID:

```json
"bootstrap_peers": [
  "/ip4/<BOOTSTRAP_IP>/tcp/30303/p2p/<BOOTSTRAP_PEER_ID>"
]
```

Rebuild and push the Docker image with the updated config.

## Step 6: Verify Deployment

Check RPC:
```bash
curl http://$BOOTSTRAP_IP:8545/health
curl -s -X POST http://$BOOTSTRAP_IP:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

Check Explorer: `http://<FAUCET_IP>:8080`
Check Faucet: `http://<FAUCET_IP>:8081`

## Step 7: Point Domains (Optional)

```bash
# DNS A records:
rpc.testnet.viri.me   → <BOOTSTRAP_IP>
explorer.testnet.viri.me → <FAUCET_IP>
faucet.testnet.viri.me   → <FAUCET_IP>
monitor.testnet.viri.me  → <MONITORING_IP>
```

## Cost Optimization

With `Standard_B1ms` (1 vCPU, 2 GB RAM):
- 7 VMs × ~$13/month = ~$91/month
- $100 credit lasts ~33 days

To reduce further, consolidate faucet+explorer onto bootstrap (5 VMs = ~$65/month).

## Cleanup

```bash
terraform destroy -auto-approve
```
