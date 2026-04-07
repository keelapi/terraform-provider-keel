data "keel_project" "existing" {
  id = "proj_abc123"
}

output "project_name" {
  value = data.keel_project.existing.name
}
