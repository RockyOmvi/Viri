# Viri Public Testnet — Deployment Guide

## Prerequisites

- **Azure subscription** with $100 credit (GitHub Education Pack)
- **Azure CLI** — [install](https://aka.ms/installazurecliwindows)
- **Terraform** — [install](https://developer.hashicorp.com/terraform/install)
- **SSH key pair** — `ssh-keygen -t ed25519 -f ~/.ssh/viri-azure -C "viri-testnet"`

## Step 1: Sign Up for Azure Student Credits

```bash
# Go to https://azure.microsoft.com/free/students
# Sign up with your school email
# Claim $100 credit (no credit card needed)
```

## Step 2: Authenticate with Azure

```bash
az login
az account set --subscription "<subscription-id>"
```

## Step 3: Configure Terraform

```bash
cd deploy/terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:
- Set `location` to your closest Azure region
- Set `ssh_public_key` to your SSH public key content
- Leave `vm_sku` as `Standard_B1s` for max credit duration

## Step 4: Deploy

```bash
terraform init
terraform plan
terraform apply -auto-approve
```

This provisions 5 VMs (~$38/month total):

| VM | Role | Services |
|----|------|----------|
| `viri-bootstrap-validator` | Bootstrap seed + validator-0 | P2P bootstrap, block producer #0 |
| `viri-validator-1` | Validator | Block producer #1 |
| `viri-validator-2` | Validator | Block producer #2 |
| `viri-validator-3` | Validator | Block producer #3 |
| `viri-services` | Public services | Faucet, Block Explorer, Prometheus/Grafana |

Terraform outputs all public IPs on completion.

## Step 5: Extract Bootstrap Peer ID

```bash
BOOTSTRAP_IP=$(terraform output -raw bootstrap_ip)
ssh -i ~/.ssh/viri-azure viriadmin@$BOOTSTRAP_IP "cat /opt/viri/config/bootstrap-id.json"
```

Save the peer ID for the next step.

## Step 6: Configure Bootstrap Peers

Update `configs/node-testnet.json` with the real bootstrap IP and peer ID, then rebuild the Docker image.

## Step 7: Verify

```bash
curl http://$BOOTSTRAP_IP:8545/health
curl -s -X POST http://$BOOTSTRAP_IP:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

Services: `http://<SERVICES_IP>:8080` (explorer), `http://<SERVICES_IP>:8081` (faucet), `http://<SERVICES_IP>:3000` (Grafana)

## Step 8: Point DNS (Optional)

```
rpc.viri.me    → <BOOTSTRAP_IP>
explorer.viri.me → <SERVICES_IP>
faucet.viri.me   → <SERVICES_IP>
```

## Cost & Duration

| Config | Monthly | $100 Credit |
|--------|---------|-------------|
| 5 × Standard_B1s | ~$38 | **~79 days** |
| 5 × Standard_B1ms | ~$65 | ~46 days |

## Cleanup

```bash
terraform destroy -auto-approve
```
