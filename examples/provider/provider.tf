terraform {
  required_providers {
    keel = {
      source  = "keelapi/keel"
      version = "~> 1.0"
    }
  }
}

provider "keel" {
  base_url = "https://api.keelapi.com" # or KEEL_BASE_URL env var
  api_key  = var.keel_api_key          # or KEEL_API_KEY env var

  # keel_organization_member needs a user token instead of the API key. User
  # tokens are short-lived: set KEEL_USER_TOKEN for each run rather than
  # user_token here.
}

variable "keel_api_key" {
  type      = string
  sensitive = true
}
