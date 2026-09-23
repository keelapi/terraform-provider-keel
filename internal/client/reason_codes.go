package client

// Reason codes Keel returns on permit decisions: in decision_details.code on
// a permit, and as the reason code of throttle responses. They are
// dot-namespaced (Shape D) and stable: Keel treats renaming one as a breaking
// change.
const (
	// Budget-driven denials and throttles.
	ReasonBudgetRequestCapExceeded    = "budget.request_cap_exceeded"
	ReasonBudgetDailyCapExceeded      = "budget.daily_cap_exceeded"
	ReasonBudgetWeeklyCapExceeded     = "budget.weekly_cap_exceeded"
	ReasonBudgetMonthlyCapExceeded    = "budget.monthly_cap_exceeded"
	ReasonBudgetQuarterlyCapExceeded  = "budget.quarterly_cap_exceeded"
	ReasonBudgetMonthlyThreshold      = "budget.monthly_threshold_exceeded"
	ReasonBudgetDailySpikeDetected    = "budget.daily_spike_detected"
	ReasonBudgetPlanQuotaExceeded     = "budget.plan_quota_exceeded"
	ReasonBudgetRateLimitExceeded     = "budget.rate_limit_exceeded"
	ReasonBudgetRateLimitThrottled    = "budget.rate_limit_throttled"
	ReasonBudgetPricingUnavailable    = "budget.pricing_unavailable"
	ReasonBudgetEvaluationUnavailable = "budget.evaluation_unavailable"
	ReasonBudgetEnvelopeRequired      = "budget_envelope_required"

	// Non-budget policy denials.
	ReasonPolicyModelNotAllowed = "policy.model_not_allowed"
	ReasonPolicyRuleDenied      = "policy.rule_denied"
	ReasonPolicyReviewRequired  = "policy.review_required"
	// ReasonPolicyExplicitAuthorizationRequired: a consequential action had
	// no applicable positive authorization (a "missing grant" deny).
	ReasonPolicyExplicitAuthorizationRequired = "policy.explicit_authorization_required"

	// Action-class policy denials.
	ReasonActionClassNotPermitted = "action_class.not_permitted"

	// Workflow declaration, enforcement and drift.
	ReasonWorkflowIntentDeclarationExceedsBudgetCap = "workflow_intent.declaration_exceeds_budget_cap"
	ReasonWorkflowIntentMaxCallsExceeded            = "workflow_intent.max_calls_exceeded"
	ReasonWorkflowIntentExpectedCallsExceeded       = "workflow_intent.expected_calls_exceeded"
	ReasonWorkflowIntentUnknownOrInactive           = "workflow_intent.unknown_or_inactive"
	ReasonWorkflowIntentIdempotencyConflict         = "workflow_intent.idempotency_conflict"
	ReasonWorkflowIntentAmendmentVersionConflict    = "workflow_intent.amendment_version_conflict"

	// Closure evidence attempted for a revoked permit.
	ReasonClosureAttemptedAfterRevocation = "closure_attempted_after_revocation"

	// Child actions inheriting a parent permit's authorization.
	ReasonInheritedParentPermitAllow   = "inherited_parent_permit.allow"
	ReasonInheritedParentPermitDeny    = "inherited_parent_permit.deny"
	ReasonInheritedParentPermitUnknown = "inherited_parent_permit.unknown"
)

// AllReasonCodes lists every reason-code constant above.
var AllReasonCodes = []string{
	ReasonBudgetRequestCapExceeded,
	ReasonBudgetDailyCapExceeded,
	ReasonBudgetWeeklyCapExceeded,
	ReasonBudgetMonthlyCapExceeded,
	ReasonBudgetQuarterlyCapExceeded,
	ReasonBudgetMonthlyThreshold,
	ReasonBudgetDailySpikeDetected,
	ReasonBudgetPlanQuotaExceeded,
	ReasonBudgetRateLimitExceeded,
	ReasonBudgetRateLimitThrottled,
	ReasonBudgetPricingUnavailable,
	ReasonBudgetEvaluationUnavailable,
	ReasonBudgetEnvelopeRequired,
	ReasonPolicyModelNotAllowed,
	ReasonPolicyRuleDenied,
	ReasonPolicyReviewRequired,
	ReasonPolicyExplicitAuthorizationRequired,
	ReasonActionClassNotPermitted,
	ReasonWorkflowIntentDeclarationExceedsBudgetCap,
	ReasonWorkflowIntentMaxCallsExceeded,
	ReasonWorkflowIntentExpectedCallsExceeded,
	ReasonWorkflowIntentUnknownOrInactive,
	ReasonWorkflowIntentIdempotencyConflict,
	ReasonWorkflowIntentAmendmentVersionConflict,
	ReasonClosureAttemptedAfterRevocation,
	ReasonInheritedParentPermitAllow,
	ReasonInheritedParentPermitDeny,
	ReasonInheritedParentPermitUnknown,
}
