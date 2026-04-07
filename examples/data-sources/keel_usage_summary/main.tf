data "keel_usage_summary" "current_month" {
  project_id = keel_project.production.id
  from       = "2026-04-01"
  to         = "2026-04-30"
}

output "total_cost" {
  value = data.keel_usage_summary.current_month.total_cost_usd
}
