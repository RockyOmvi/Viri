variable "tenancy_ocid" {
  description = "Oracle Cloud Infrastructure tenancy OCID"
  type        = string
  sensitive   = true
}

variable "user_ocid" {
  description = "Oracle Cloud Infrastructure user OCID"
  type        = string
  sensitive   = true
}

variable "fingerprint" {
  description = "Fingerprint of the OCI API key"
  type        = string
  sensitive   = true
}

variable "private_key_path" {
  description = "Path to the OCI API private key file"
  type        = string
  sensitive   = true
}

variable "region" {
  description = "OCI region (e.g. ap-mumbai-1, eu-frankfurt-1)"
  type        = string
  default     = "ap-mumbai-1"
}

variable "compartment_ocid" {
  description = "OCI compartment OCID"
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "Public SSH key content for VM access"
  type        = string
  sensitive   = true
}

variable "domain" {
  description = "Domain name for TLS (optional, e.g. viri-testnet.io)"
  type        = string
  default     = ""
}

variable "instance_shape" {
  description = "Oracle Cloud instance shape"
  type        = string
  default     = "VM.Standard.A1.Flex"
}

variable "instance_ocpus" {
  description = "Number of OCPUs per instance"
  type        = number
  default     = 4
}

variable "instance_memory_gb" {
  description = "Memory in GB per instance"
  type        = number
  default     = 24
}

variable "faucet_wallet_key" {
  description = "Hex-encoded private key for the faucet wallet"
  type        = string
  sensitive   = true
  default     = ""
}
