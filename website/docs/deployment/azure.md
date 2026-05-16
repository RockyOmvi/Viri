# Azure Deployment

Provision 5 Azure VMs using Terraform for a public testnet.

## Prerequisites

- Azure account with $100 free credit
- Terraform installed
- Azure CLI (`az login`)

## Steps

```bash
# Navigate to terraform config
cd deploy/terraform

# Copy and fill variables
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your Azure subscription details

# Initialize and apply
terraform init
terraform apply -auto-approve
```

## VM Roles

| VM | Role | Spec |
|----|------|------|
| bootstrap | P2P entry + RPC | Standard_B2s |
| validator-0 | Consensus | Standard_B2s |
| validator-1 | Consensus | Standard_B2s |
| validator-2 | Consensus | Standard_B2s |
| validator-3 | Consensus | Standard_B2s |
| faucet | Faucet service | Standard_B2s |

## Post-Deployment

After Terraform completes, SSH into each VM and run the standalone provision script:

```bash
# On bootstrap
ssh viri@<bootstrap-ip>
sudo bash /opt/viri/standalone-provision.sh bootstrap

# On each validator
ssh viri@<validator-N-ip>
sudo bash /opt/viri/standalone-provision.sh validator \
  --index N \
  --bootstrap-addr <bootstrap-ip> \
  --bootstrap-peer-id <peer-id>
```

## DNS Setup

Configure Cloudflare DNS to point your domain to the bootstrap node's public IP.
