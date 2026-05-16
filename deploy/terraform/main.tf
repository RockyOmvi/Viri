terraform {
  required_version = ">= 1.5"
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

# --- Variables ---
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
  description = "Azure VM SKU (size). B2s=2vCPU/4GB ~$30/mo, B1ms=1vCPU/2GB ~$13/mo"
  type        = string
  default     = "Standard_B2s"
}

# --- Locals ---
locals {
  network_cidr  = "10.0.0.0/16"
  subnet_cidr   = "10.0.1.0/24"
  instance_count = 5
  instance_names = ["bootstrap", "validator-0", "validator-1", "validator-2", "faucet"]
  resource_prefix = "viri-testnet"
  tags = {
    Environment = "testnet"
    Project     = "viri-blockchain"
    ManagedBy   = "terraform"
  }
}

# --- Resource Group ---
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

# --- Network Security Group (firewall rules) ---
resource "azurerm_network_security_group" "main" {
  name                = "${local.resource_prefix}-nsg"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tags                = local.tags
}

resource "azurerm_network_security_rule" "ssh" {
  name                        = "SSH"
  priority                    = 100
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "22"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "p2p" {
  name                        = "P2P"
  priority                    = 101
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "30303"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "json_rpc" {
  name                        = "JSON-RPC"
  priority                    = 102
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "8545"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "rest_api" {
  name                        = "REST-API"
  priority                    = 103
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "8546"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "explorer" {
  name                        = "Explorer"
  priority                    = 104
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "8080"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "faucet" {
  name                        = "Faucet"
  priority                    = 105
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "8081"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "metrics" {
  name                        = "Metrics"
  priority                    = 106
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "9090"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "internal_gossip_tcp" {
  name                        = "Internal-Gossip-TCP"
  priority                    = 200
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "7946"
  source_address_prefix       = local.subnet_cidr
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

resource "azurerm_network_security_rule" "internal_gossip_udp" {
  name                        = "Internal-Gossip-UDP"
  priority                    = 201
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Udp"
  source_port_range           = "*"
  destination_port_range      = "7946"
  source_address_prefix       = local.subnet_cidr
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.main.name
  network_security_group_name = azurerm_network_security_group.main.name
}

# --- Public IPs ---
resource "azurerm_public_ip" "vms" {
  count               = local.instance_count
  name                = "${local.resource_prefix}-pip-${local.instance_names[count.index]}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  allocation_method   = "Static"
  sku                 = "Standard"
  domain_name_label   = "viri-${local.instance_names[count.index]}"
  tags                = local.tags
}

# --- Network Interfaces ---
resource "azurerm_network_interface" "vms" {
  count               = local.instance_count
  name                = "${local.resource_prefix}-nic-${local.instance_names[count.index]}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tags                = local.tags

  ip_configuration {
    name                          = "primary"
    subnet_id                     = azurerm_subnet.main.id
    private_ip_address_allocation = "Static"
    private_ip_address            = cidrhost(local.subnet_cidr, count.index + 10)
    public_ip_address_id          = azurerm_public_ip.vms[count.index].id
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
    sku       = "server"
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
  }))

  boot_diagnostics {
    storage_account_uri = null
  }

  identity {
    type = "SystemAssigned"
  }
}

# --- Outputs ---
output "instance_ips" {
  value = {
    for i, name in local.instance_names :
    name => azurerm_public_ip.vms[i].ip_address
  }
  description = "Map of instance names to public IPs"
}

output "instance_fqdns" {
  value = {
    for i, name in local.instance_names :
    name => azurerm_public_ip.vms[i].fqdn
  }
  description = "Map of instance names to FQDNs"
}

output "bootstrap_ip" {
  value = azurerm_public_ip.vms[0].ip_address
}

output "bootstrap_fqdn" {
  value = azurerm_public_ip.vms[0].fqdn
}

output "validator_ips" {
  value = [
    for i in range(1, 4) : azurerm_public_ip.vms[i].ip_address
  ]
}

output "faucet_ip" {
  value = azurerm_public_ip.vms[4].ip_address
}

output "resource_group" {
  value = azurerm_resource_group.main.name
}

output "ssh_command" {
  value = {
    for i, name in local.instance_names :
    name => "ssh ${var.admin_username}@${azurerm_public_ip.vms[i].ip_address}"
  }
}
