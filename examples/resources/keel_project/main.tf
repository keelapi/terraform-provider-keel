resource "keel_project" "production" {
  name        = "production-api"
  description = "Production AI services"

  settings {
    default_provider = "openai"
    default_model    = "gpt-4o"
    budget_limit_usd = 1000.00
    rate_limit_rpm   = 600
  }
}

output "project_id" {
  value = keel_project.production.id
}
