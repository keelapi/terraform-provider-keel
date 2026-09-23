# Requires the provider's user_token (KEEL_USER_TOKEN) for an owner or admin of
# the organization: Keel's organization member routes do not accept API keys.

variable "org_id" {
  type        = string
  description = "Keel organization ID."
}

variable "user_id" {
  type        = string
  description = "Keel user ID to add to the organization."
}

resource "keel_organization_member" "member" {
  org_id  = var.org_id
  user_id = var.user_id
  role    = "member"
}

output "organization_member_id" {
  value = keel_organization_member.member.id
}
