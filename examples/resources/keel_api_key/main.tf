resource "keel_api_key" "backend" {
  name        = "backend-service"
  description = "Key for the backend service"
  scope       = "client"
}

output "api_key_id" {
  value = keel_api_key.backend.id
}

output "api_key_project_id" {
  value = keel_api_key.backend.project_id
}

output "api_key_prefix" {
  value = keel_api_key.backend.prefix
}
