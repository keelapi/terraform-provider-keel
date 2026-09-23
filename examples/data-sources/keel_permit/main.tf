data "keel_permit" "last_denied" {
  decision = "deny"
  limit    = 1
}

data "keel_permit" "held_for_review" {
  decision = "review"
  limit    = 20
}
