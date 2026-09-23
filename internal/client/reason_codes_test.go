package client

import (
	"sort"
	"testing"
)

// keelReasonCodes is the reason-code lexicon the Keel API emits on permit
// decisions (27 codes), plus the positive-authority deny code.
var keelReasonCodes = []string{
	"budget.request_cap_exceeded",
	"budget.daily_cap_exceeded",
	"budget.weekly_cap_exceeded",
	"budget.monthly_cap_exceeded",
	"budget.quarterly_cap_exceeded",
	"budget.monthly_threshold_exceeded",
	"budget.daily_spike_detected",
	"budget.plan_quota_exceeded",
	"budget.rate_limit_exceeded",
	"budget.rate_limit_throttled",
	"budget.pricing_unavailable",
	"budget.evaluation_unavailable",
	"budget_envelope_required",
	"policy.model_not_allowed",
	"policy.rule_denied",
	"policy.review_required",
	"action_class.not_permitted",
	"workflow_intent.declaration_exceeds_budget_cap",
	"workflow_intent.max_calls_exceeded",
	"workflow_intent.expected_calls_exceeded",
	"workflow_intent.unknown_or_inactive",
	"workflow_intent.idempotency_conflict",
	"workflow_intent.amendment_version_conflict",
	"closure_attempted_after_revocation",
	"inherited_parent_permit.allow",
	"inherited_parent_permit.deny",
	"inherited_parent_permit.unknown",
	"policy.explicit_authorization_required",
}

func TestAllReasonCodesMatchesKeelLexicon(t *testing.T) {
	got := append([]string(nil), AllReasonCodes...)
	want := append([]string(nil), keelReasonCodes...)
	sort.Strings(got)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("AllReasonCodes has %d codes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllReasonCodes (sorted) [%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAllReasonCodesKeepsV1Constants(t *testing.T) {
	// The eleven constants shipped in v1.0.x keep their names and values.
	v1 := map[string]string{
		ReasonBudgetRequestCapExceeded: "budget.request_cap_exceeded",
		ReasonBudgetDailyCapExceeded:   "budget.daily_cap_exceeded",
		ReasonBudgetMonthlyCapExceeded: "budget.monthly_cap_exceeded",
		ReasonBudgetMonthlyThreshold:   "budget.monthly_threshold_exceeded",
		ReasonBudgetDailySpikeDetected: "budget.daily_spike_detected",
		ReasonBudgetRateLimitExceeded:  "budget.rate_limit_exceeded",
		ReasonBudgetRateLimitThrottled: "budget.rate_limit_throttled",
		ReasonBudgetPricingUnavailable: "budget.pricing_unavailable",
		ReasonPolicyModelNotAllowed:    "policy.model_not_allowed",
		ReasonPolicyRuleDenied:         "policy.rule_denied",
		ReasonPolicyReviewRequired:     "policy.review_required",
	}
	for got, want := range v1 {
		if got != want {
			t.Errorf("constant = %q, want %q", got, want)
		}
	}
}
