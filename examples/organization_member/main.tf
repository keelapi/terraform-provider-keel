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
