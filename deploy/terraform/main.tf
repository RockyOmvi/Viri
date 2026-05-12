terraform {
  required_version = ">= 1.5"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 6.0"
    }
  }
}

variable "tenancy_ocid"     { type = string }
variable "user_ocid"        { type = string }
variable "fingerprint"      { type = string }
variable "private_key_path" { type = string }
variable "region"           { type = string }
variable "compartment_ocid" { type = string }
variable "ssh_public_key"   { type = string }
variable "domain"           { type = string }

locals {
  network_cidr = "10.0.0.0/16"
  subnet_cidr  = "10.0.1.0/24"
  instance_count = 5
  instance_names = ["bootstrap", "validator-0", "validator-1", "validator-2", "faucet"]
}

provider "oci" {
  tenancy_ocid     = var.tenancy_ocid
  user_ocid        = var.user_ocid
  fingerprint      = var.fingerprint
  private_key_path = var.private_key_path
  region           = var.region
}

resource "oci_core_vcn" "viri_testnet" {
  compartment_id = var.compartment_ocid
  display_name   = "viri-testnet-vcn"
  cidr_block     = local.network_cidr
  dns_label      = "viri"
}

resource "oci_core_subnet" "viri_testnet" {
  compartment_id    = var.compartment_ocid
  vcn_id            = oci_core_vcn.viri_testnet.id
  display_name      = "viri-testnet-subnet"
  cidr_block        = local.subnet_cidr
  dns_label         = "viri"
  security_list_ids = [oci_core_security_list.viri_testnet.id]
}

resource "oci_core_internet_gateway" "viri_testnet" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.viri_testnet.id
  display_name   = "viri-testnet-igw"
}

resource "oci_core_default_route_table" "viri_testnet" {
  manage_default_resource_id = oci_core_vcn.viri_testnet.default_route_table_id
  display_name               = "viri-testnet-rt"

  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = oci_core_internet_gateway.viri_testnet.id
  }
}

# Security list — open ports for blockchain node
resource "oci_core_security_list" "viri_testnet" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.viri_testnet.id
  display_name   = "viri-testnet-sl"

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    description = "SSH"
    tcp_options {
      min = 22
      max = 22
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    description = "P2P"
    tcp_options {
      min = 30303
      max = 30303
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    description = "JSON-RPC"
    tcp_options {
      min = 8545
      max = 8545
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    description = "REST API"
    tcp_options {
      min = 8546
      max = 8546
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    description = "Explorer"
    tcp_options {
      min = 8080
      max = 8080
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    description = "Faucet"
    tcp_options {
      min = 8081
      max = 8081
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    description = "Metrics"
    tcp_options {
      min = 9090
      max = 9090
    }
  }

  ingress_security_rules {
    protocol = "6"
    source   = local.network_cidr
    description = "Internal gossip"
    tcp_options {
      min = 7946
      max = 7946
    }
  }

  ingress_security_rules {
    protocol = "17"
    source   = local.network_cidr
    description = "Serf LAN"
    tcp_options {
      min = 7946
      max = 7946
    }
  }
}

# Static public IPs
resource "oci_core_public_ip" "viri_testnet" {
  count          = local.instance_count
  compartment_id = var.compartment_ocid
  display_name   = "viri-${local.instance_names[count.index]}-ip"
  lifetime       = "RESERVED"
}

# Compute instances
resource "oci_core_instance" "viri_testnet" {
  count                = local.instance_count
  compartment_id       = var.compartment_ocid
  display_name         = "viri-${local.instance_names[count.index]}"
  availability_domain  = data.oci_identity_availability_domains.ads.availability_domains[0].name
  shape                = "VM.Standard.A1.Flex"
  shape_config {
    ocpus         = 4
    memory_in_gbs = 24
  }

  source_details {
    source_type = "image"
    source_id   = data.oci_core_images.ubuntu.id
  }

  metadata = {
    ssh_authorized_keys = var.ssh_public_key
    user_data = base64encode(templatefile("${path.module}/cloud-init.yaml", {
      instance_name    = local.instance_names[count.index]
      domain           = var.domain
      faucet_wallet_key = var.faucet_wallet_key
    }))
  }

  create_vnic_details {
    display_name      = "viri-${local.instance_names[count.index]}-vnic"
    subnet_id         = oci_core_subnet.viri_testnet.id
    private_ip        = cidrhost(local.subnet_cidr, count.index + 10)
    public_ip         = oci_core_public_ip.viri_testnet[count.index].ip_address
    skip_source_dest_check = true
  }
}

data "oci_identity_availability_domains" "ads" {
  compartment_id = var.compartment_ocid
}

data "oci_core_images" "ubuntu" {
  compartment_id           = var.compartment_ocid
  operating_system         = "Canonical Ubuntu"
  operating_system_version = "24.04"
  shape                    = "VM.Standard.A1.Flex"
  sort_by                  = "TIMECREATED"
  sort_order               = "DESC"
}

output "instance_ips" {
  value = {
    for i, name in local.instance_names :
    name => oci_core_public_ip.viri_testnet[i].ip_address
  }
}

output "bootstrap_ip" {
  value = oci_core_public_ip.viri_testnet[0].ip_address
}

output "validator_ips" {
  value = [
    for i in range(1, 4) : oci_core_public_ip.viri_testnet[i].ip_address
  ]
}

output "faucet_ip" {
  value = oci_core_public_ip.viri_testnet[4].ip_address
}
