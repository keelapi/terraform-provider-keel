terraform {
  required_providers {
    keel = {
      source  = "keelapi/keel"
      version = "~> 0.1"
    }
  }
}

provider "keel" {
  base_url = "https://api.keelapi.com" # or KEEL_BASE_URL env var
  api_key  = var.keel_api_key          # or KEEL_API_KEY env var
}

variable "keel_api_key" {
  type      = string
  sensitive = true
}
