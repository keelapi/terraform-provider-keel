resource "keel_provider_key" "openai_prod" {
  project_id = keel_project.production.id
  provider   = "openai"
  key_value  = var.openai_api_key
  enabled    = true
}

variable "openai_api_key" {
  type      = string
  sensitive = true
}
