terraform {
  required_providers {
    keel = {
      source  = "keelapi/keel"
      version = "~> 0.1"
    }
  }
}

provider "keel" {}

variable "org_id" {
  type        = string
  description = "Keel organization ID."
}

variable "user_id" {
  type        = string
  description = "Keel user ID to add to the organization."
}

variable "member_role" {
  type        = string
  description = "Organization role to assign."
  default     = "member"
}

variable "owner_grant_approvals" {
  type = map(object({
    approved_by = string
    reason      = optional(string)
  }))
  description = "Approval annotations for owner grants, keyed by org_id/user_id."
  default     = {}
}

resource "keel_api_key" "automation" {
  name        = "opa-gated-automation"
  description = "Example key managed through an OPA-gated Terraform pipeline"
  scope       = "client"
}

resource "keel_organization_member" "reviewer" {
  org_id  = var.org_id
  user_id = var.user_id
  role    = var.member_role
}

data "keel_permit" "recent_denials" {
  decision = "deny"
  limit    = 5
}

output "api_key_id" {
  value = keel_api_key.automation.id
}

output "organization_member_id" {
  value = keel_organization_member.reviewer.id
}
