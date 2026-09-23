package client

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
)

// authFailureRateLimitedCode is the 429 code Keel returns in place of a 401 or
// 403 once a client has sent too many requests with a bad credential. Retrying
// cannot succeed and only extends the lockout, so it is never retried.
const authFailureRateLimitedCode = "auth_failure_rate_limited"

// APIError represents a non-2xx response from the Keel API.
type APIError struct {
	StatusCode int
	// Code, Message and Field come from Keel's error envelope,
	// {"error": {"code": ..., "message": ..., "field": ...}}. Message also holds
	// the text of a bare {"detail": "..."} body. They are empty when the body
	// has neither shape.
	Code    string
	Message string
	Field   string
	Body    []byte
}

func newAPIError(statusCode int, body []byte) *APIError {
	env := parseErrorEnvelope(body)
	return &APIError{
		StatusCode: statusCode,
		Code:       env.code(),
		Message:    env.message(),
		Field:      env.field(),
		Body:       body,
	}
}

func (e *APIError) Error() string {
	return describeError("API error", e.StatusCode, e.Code, e.Message, e.Field, e.Body)
}

// ThrottledError is returned when the API responds with HTTP 429 and the
// request was not retried, or all retries have been exhausted.
type ThrottledError struct {
	RetryAfterSeconds int
	// Code and Message come from Keel's error envelope, for example
	// "rate_limited" or "auth_failure_rate_limited".
	Code    string
	Message string
	// PermitID and ReasonCode are set when the body carries them. ReasonCode
	// falls back to Code.
	PermitID   string
	ReasonCode string
	Body       []byte
}

func newThrottledError(retryAfterHeader string, body []byte) *ThrottledError {
	env := parseErrorEnvelope(body)
	e := &ThrottledError{
		RetryAfterSeconds: parseRetryAfter(retryAfterHeader, body),
		Code:              env.code(),
		Message:           env.message(),
		Body:              body,
	}
	if env.Permit != nil {
		e.PermitID = env.Permit.PermitID
		e.ReasonCode = env.Permit.ReasonCode
	}
	if env.Error != nil {
		if env.Error.Details != nil && env.Error.Details.ReasonCode != "" {
			e.ReasonCode = env.Error.Details.ReasonCode
		} else if env.Error.ReasonCode != "" {
			e.ReasonCode = env.Error.ReasonCode
		}
	}
	if e.ReasonCode == "" {
		e.ReasonCode = e.Code
	}
	return e
}

func (e *ThrottledError) Error() string {
	return fmt.Sprintf("%s (retry after %ds)",
		describeError("API rate limit", http.StatusTooManyRequests, e.Code, e.Message, "", e.Body),
		e.RetryAfterSeconds,
	)
}

// retryable reports whether waiting RetryAfterSeconds and sending the request
// again can succeed.
func (e *ThrottledError) retryable() bool {
	return e.Code != authFailureRateLimitedCode && e.RetryAfterSeconds <= MaxThrottleWaitSeconds
}

// errorEnvelope covers the error bodies Keel sends:
//
//   - {"error": {"code", "message", "field"?, ...}}: the standard envelope.
//     Some errors flatten extra details into "error" (a 429's
//     retry_after_seconds); others nest them under "error.details".
//   - {"detail": "..."}: requests that never reach a Keel route, such as an
//     unknown path.
//   - {"permit": {...}}: the legacy throttle shape, still accepted.
type errorEnvelope struct {
	Error *struct {
		Code              string   `json:"code"`
		Message           string   `json:"message"`
		Field             string   `json:"field"`
		RetryAfterSeconds *float64 `json:"retry_after_seconds"`
		ReasonCode        string   `json:"reason_code"`
		Details           *struct {
			RetryAfterSeconds *float64 `json:"retry_after_seconds"`
			ReasonCode        string   `json:"reason_code"`
		} `json:"details"`
	} `json:"error"`
	Detail json.RawMessage `json:"detail"`
	Permit *struct {
		PermitID      string `json:"permit_id"`
		ReasonCode    string `json:"reason_code"`
		OutcomeDetail *struct {
			RetryAfterSeconds *float64 `json:"retry_after_seconds"`
		} `json:"outcome_detail"`
	} `json:"permit"`
}

// parseErrorEnvelope decodes whichever parts of an error body it recognizes.
// Any field may be missing; a body that is not JSON yields an empty envelope.
func parseErrorEnvelope(body []byte) errorEnvelope {
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return errorEnvelope{}
	}
	return env
}

func (env errorEnvelope) code() string {
	if env.Error == nil {
		return ""
	}
	return env.Error.Code
}

func (env errorEnvelope) message() string {
	if env.Error != nil && env.Error.Message != "" {
		return env.Error.Message
	}
	var detail string
	if len(env.Detail) > 0 && json.Unmarshal(env.Detail, &detail) == nil {
		return detail
	}
	return ""
}

func (env errorEnvelope) field() string {
	if env.Error == nil {
		return ""
	}
	return env.Error.Field
}

// retryAfterSeconds returns the wait the body asks for, preferring
// error.details.retry_after_seconds, then error.retry_after_seconds, then the
// legacy permit.outcome_detail.retry_after_seconds. It returns 0 when none is
// set.
func (env errorEnvelope) retryAfterSeconds() int {
	var candidates []*float64
	if env.Error != nil {
		if env.Error.Details != nil {
			candidates = append(candidates, env.Error.Details.RetryAfterSeconds)
		}
		candidates = append(candidates, env.Error.RetryAfterSeconds)
	}
	if env.Permit != nil && env.Permit.OutcomeDetail != nil {
		candidates = append(candidates, env.Permit.OutcomeDetail.RetryAfterSeconds)
	}
	for _, seconds := range candidates {
		if seconds != nil && *seconds > 0 {
			return int(math.Ceil(*seconds))
		}
	}
	return 0
}

func describeError(prefix string, statusCode int, code, message, field string, body []byte) string {
	var text string
	switch {
	case code != "" && message != "":
		text = code + ": " + message
	case message != "":
		text = message
	case code != "":
		text = code
	default:
		text = strings.TrimSpace(string(body))
		if text == "" {
			text = "(empty response body)"
		}
	}
	if field != "" {
		text += fmt.Sprintf(" (field %q)", field)
	}
	return fmt.Sprintf("%s (status %d): %s", prefix, statusCode, text)
}
