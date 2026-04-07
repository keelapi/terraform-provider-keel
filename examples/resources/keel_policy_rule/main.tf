resource "keel_policy_rule" "cost_cap" {
  name       = "daily-cost-cap"
  priority   = 10

  condition {
    field    = "resource.attributes.estimated_cost_usd"
    operator = "greater_than"
    value    = "50.00"
  }

  action  = "deny"
  reason  = "Single request cost exceeds $50 daily cap"
  enabled = true
}

resource "keel_policy_rule" "model_allowlist" {
  name       = "approved-models-only"
  priority   = 20

  condition {
    field    = "resource.attributes.model"
    operator = "not_in"
    value    = jsonencode(["gpt-4o", "gpt-4o-mini", "claude-3-5-sonnet"])
  }

  action  = "deny"
  reason  = "Model not in approved list"
  enabled = true
}
