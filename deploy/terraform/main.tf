terraform {
  required_version = ">= 1.5, < 2.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}

provider "azurerm" {
  features {}
}

variable "location" {
  description = "Azure region (eastus, westeurope, southeastasia, etc.)"
  type        = string
  default     = "eastus"
}

variable "ssh_public_key" {
  description = "Public SSH key content for VM access"
  type        = string
  sensitive   = true
}

variable "admin_username" {
  description = "Admin username for VMs"
  type        = string
  default     = "viriadmin"
}

variable "domain" {
  description = "Domain name for TLS (optional)"
  type        = string
  default     = ""
}

variable "faucet_wallet_key" {
  description = "Hex-encoded private key for the faucet wallet"
  type        = string
  sensitive   = true
  default     = ""
}

variable "vm_sku" {
  description = "Azure VM SKU. B1s=1vCPU/1GB ~$7.50/mo, B1ms=1vCPU/2GB ~$13/mo"
  type        = string
  default     = "Standard_B1s"
}

variable "viri_key_passphrase" {
  description = "Passphrase for encrypted validator keys"
  type        = string
  sensitive   = true
  default     = ""
}

variable "bootstrap_key" {
  description = "Hex-encoded private key for the bootstrap validator"
  type        = string
  sensitive   = true
  default     = ""
}

variable "validator_keys" {
  description = "Map of validator names to hex-encoded private keys"
  type        = map(string)
  sensitive   = true
  default     = {}
}

variable "admin_cidrs" {
  description = "CIDR blocks allowed to access SSH, JSON-RPC, and Metrics (empty = no external access)"
  type        = list(string)
  default     = []
}

locals {
  network_cidr  = "10.0.0.0/16"
  subnet_cidr   = "10.0.1.0/24"
  instance_count = 3
  instance_names = ["bootstrap-validator", "validator-1", "services"]
  resource_prefix = "viri-testnet"
  tags = {
    Environment = "testnet"
    Project     = "viri-blockchain"
    ManagedBy   = "terraform"
  }
}

resource "azurerm_resource_group" "main" {
  name     = "${local.resource_prefix}-rg"
  location = var.location
  tags     = local.tags
}

# --- Virtual Network ---
resource "azurerm_virtual_network" "main" {
  name                = "${local.resource_prefix}-vnet"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  address_space       = [local.network_cidr]
  tags                = local.tags
}

resource "azurerm_subnet" "main" {
  name                 = "${local.resource_prefix}-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [local.subnet_cidr]
}

# --- Network Security Group with all rules inline (single API call) ---
resource "azurerm_network_security_group" "main" {
  name                = "${local.resource_prefix}-nsg"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tags                = local.tags

  security_rule {
    name                       = "SSH"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefixes    = length(var.admin_cidrs) > 0 ? var.admin_cidrs : ["10.0.0.0/16"]
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "P2P"
    priority                   = 101
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "30303"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "JSON-RPC"
    priority                   = 102
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "8545"
    source_address_prefixes    = length(var.admin_cidrs) > 0 ? var.admin_cidrs : ["10.0.0.0/16"]
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "REST-API"
    priority                   = 103
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "8546"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "Explorer"
    priority                   = 104
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "8080"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "Faucet"
    priority                   = 105
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "8081"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "Metrics"
    priority                   = 106
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "9090"
    source_address_prefixes    = length(var.admin_cidrs) > 0 ? var.admin_cidrs : ["10.0.0.0/16"]
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "HTTP"
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "80"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "HTTPS"
    priority                   = 111
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "Internal-Gossip-TCP"
    priority                   = 200
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "7946"
    source_address_prefix      = local.subnet_cidr
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "Internal-Gossip-UDP"
    priority                   = 201
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Udp"
    source_port_range          = "*"
    destination_port_range     = "7946"
    source_address_prefix      = local.subnet_cidr
    destination_address_prefix = "*"
  }
}

# --- Public IPs (All 3) ---
locals {
  public_ip_indices = [0, 1, 2]
  public_ip_names   = [for i in local.public_ip_indices : local.instance_names[i]]
}

resource "azurerm_public_ip" "vms" {
  count               = length(local.public_ip_indices)
  name                = "${local.resource_prefix}-pip-${local.public_ip_names[count.index]}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = local.tags
}

locals {
  vm_to_pip = {
    for i in range(local.instance_count) :
    i => contains(local.public_ip_indices, i) ? index(local.public_ip_indices, i) : null
  }
}

# --- Network Interfaces ---
resource "azurerm_network_interface" "vms" {
  count               = local.instance_count
  name                = "${local.resource_prefix}-nic-${local.instance_names[count.index]}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tags                = local.tags

  dynamic "ip_configuration" {
    for_each = local.vm_to_pip[count.index] != null ? [1] : []
    content {
      name                          = "primary"
      subnet_id                     = azurerm_subnet.main.id
      private_ip_address_allocation = "Static"
      private_ip_address            = cidrhost(local.subnet_cidr, count.index + 10)
      public_ip_address_id          = azurerm_public_ip.vms[local.vm_to_pip[count.index]].id
    }
  }

  dynamic "ip_configuration" {
    for_each = local.vm_to_pip[count.index] == null ? [1] : []
    content {
      name                          = "primary"
      subnet_id                     = azurerm_subnet.main.id
      private_ip_address_allocation = "Static"
      private_ip_address            = cidrhost(local.subnet_cidr, count.index + 10)
    }
  }
}

# --- VMs ---
resource "azurerm_linux_virtual_machine" "vms" {
  count                = local.instance_count
  name                 = "viri-${local.instance_names[count.index]}"
  location             = azurerm_resource_group.main.location
  resource_group_name  = azurerm_resource_group.main.name
  size                 = var.vm_sku
  admin_username       = var.admin_username
  network_interface_ids = [azurerm_network_interface.vms[count.index].id]
  tags                 = local.tags

  admin_ssh_key {
    username   = var.admin_username
    public_key = var.ssh_public_key
  }

  source_image_reference {
    publisher = "canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = length(regexall("pts", var.vm_sku)) > 0 ? "server-arm64" : "server"
    version   = "latest"
  }

  os_disk {
    name                 = "${local.resource_prefix}-disk-${local.instance_names[count.index]}"
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
    disk_size_gb         = 64
  }

  user_data = base64encode(templatefile("${path.module}/cloud-init.yaml", {
    instance_name     = local.instance_names[count.index]
    domain            = var.domain
    faucet_wallet_key = var.faucet_wallet_key
    genesis_json_b64  = base64encode(file("${path.module}/../../configs/genesis/testnet.json"))
    bootstrap_private_ip = "10.0.1.10"
    domain               = var.domain
    viri_key_passphrase  = var.viri_key_passphrase
    bootstrap_key        = var.bootstrap_key
    validator_keys       = var.validator_keys
  }))

  boot_diagnostics {
    storage_account_uri = null
  }

  identity {
    type = "SystemAssigned"
  }
}

# --- NSG Association (subnet-level — applies to all VMs in subnet) ---
resource "azurerm_subnet_network_security_group_association" "main" {
  subnet_id                 = azurerm_subnet.main.id
  network_security_group_id = azurerm_network_security_group.main.id
}

# --- Outputs ---
output "instance_ips" {
  value = {
    for i, name in local.instance_names :
    name => local.vm_to_pip[i] != null ? azurerm_public_ip.vms[local.vm_to_pip[i]].ip_address : azurerm_network_interface.vms[i].private_ip_address
  }
}

output "instance_fqdns" {
  value = {
    for i, name in local.instance_names :
    name => local.vm_to_pip[i] != null ? azurerm_public_ip.vms[local.vm_to_pip[i]].fqdn : "N/A (private IP only)"
  }
}

output "bootstrap_ip" {
  value = azurerm_public_ip.vms[local.vm_to_pip[0]].ip_address
}

output "bootstrap_fqdn" {
  value = azurerm_public_ip.vms[local.vm_to_pip[0]].fqdn
}

output "validator_ips" {
  value = {
    for i in range(1, 3) : local.instance_names[i] => local.vm_to_pip[i] != null ? azurerm_public_ip.vms[local.vm_to_pip[i]].ip_address : azurerm_network_interface.vms[i].private_ip_address
  }
}

output "services_ip" {
  value = azurerm_public_ip.vms[local.vm_to_pip[2]].ip_address
}

output "resource_group" {
  value = azurerm_resource_group.main.name
}

output "ssh_command" {
  value = {
    for i, name in local.instance_names :
    name => local.vm_to_pip[i] != null ? "ssh ${var.admin_username}@${azurerm_public_ip.vms[local.vm_to_pip[i]].ip_address}" : "ssh ${var.admin_username}@${azurerm_network_interface.vms[i].private_ip_address} (via VPN/bastion)"
  }
}
