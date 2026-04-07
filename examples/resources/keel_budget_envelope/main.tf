resource "keel_budget_envelope" "q2_budget" {
  project_id   = keel_project.production.id
  name         = "Q2-2026-AI-Budget"
  amount_usd   = 10000.00
  period       = "monthly"
  alert_at_pct = [50, 75, 90]
  hard_cap     = true
}
