resource "keel_api_key" "backend" {
  project_id = keel_project.production.id
  name       = "backend-service"
  scope      = "project"
}

output "api_key_id" {
  value = keel_api_key.backend.id
}

output "api_key_prefix" {
  value = keel_api_key.backend.prefix
}
