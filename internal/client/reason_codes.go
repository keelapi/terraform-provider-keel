package client

// Shape D reason codes (V1.16.0+). All new permits carry these
// dot-namespaced codes in the reason_code field.
const (
	ReasonBudgetRequestCapExceeded    = "budget.request_cap_exceeded"
	ReasonBudgetDailyCapExceeded      = "budget.daily_cap_exceeded"
	ReasonBudgetMonthlyCapExceeded    = "budget.monthly_cap_exceeded"
	ReasonBudgetMonthlyThreshold      = "budget.monthly_threshold_exceeded"
	ReasonBudgetDailySpikeDetected    = "budget.daily_spike_detected"
	ReasonBudgetRateLimitExceeded     = "budget.rate_limit_exceeded"
	ReasonBudgetRateLimitThrottled    = "budget.rate_limit_throttled"
	ReasonBudgetPricingUnavailable    = "budget.pricing_unavailable"
	ReasonPolicyModelNotAllowed       = "policy.model_not_allowed"
	ReasonPolicyRuleDenied            = "policy.rule_denied"
	ReasonPolicyReviewRequired        = "policy.review_required"
)
