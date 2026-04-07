resource "keel_routing_config" "multi_provider" {
  project_id = keel_project.production.id

  route {
    provider = "openai"
    model    = "gpt-4o"
    weight   = 70
    priority = 1
  }

  route {
    provider = "anthropic"
    model    = "claude-3-5-sonnet"
    weight   = 30
    priority = 2
  }

  fallback_provider = "openai"
  fallback_model    = "gpt-4o-mini"
}
