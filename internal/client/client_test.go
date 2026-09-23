package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Response bodies below mirror what the Keel API returns for these statuses:
// the standard {"error": {...}} envelope, with any details flattened into
// "error" (for example retry_after_seconds on an authentication-failure 429).

func TestDoRequest_429Retry_Success(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":"rate_limited","message":"Too many requests"}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	body, err := c.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestDoRequest_429Exhaust(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"code":"rate_limited","message":"Too many requests"}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	_, err := c.Get(context.Background(), "/test")
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}

	var throttled *ThrottledError
	if !errors.As(err, &throttled) {
		t.Fatalf("expected ThrottledError, got %T: %v", err, err)
	}
	if throttled.RetryAfterSeconds != 1 {
		t.Errorf("RetryAfterSeconds = %d, want 1", throttled.RetryAfterSeconds)
	}
	if throttled.Code != "rate_limited" {
		t.Errorf("Code = %q, want rate_limited", throttled.Code)
	}
	if throttled.Message != "Too many requests" {
		t.Errorf("Message = %q, want %q", throttled.Message, "Too many requests")
	}
	if throttled.ReasonCode != "rate_limited" {
		t.Errorf("ReasonCode = %q, want rate_limited (falls back to the error code)", throttled.ReasonCode)
	}
	want := "API rate limit (status 429): rate_limited: Too many requests (retry after 1s)"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDoRequest_429LegacyPermitShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"permit":{"permit_id":"pmt_123","decision":"throttled","reason_code":"budget.rate_limit_throttled","outcome_detail":{"retry_after_seconds":1}}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	_, err := c.Get(context.Background(), "/test")

	var throttled *ThrottledError
	if !errors.As(err, &throttled) {
		t.Fatalf("expected ThrottledError, got %T: %v", err, err)
	}
	if throttled.PermitID != "pmt_123" {
		t.Errorf("PermitID = %q, want pmt_123", throttled.PermitID)
	}
	if throttled.ReasonCode != "budget.rate_limit_throttled" {
		t.Errorf("ReasonCode = %q, want budget.rate_limit_throttled", throttled.ReasonCode)
	}
}

func TestDoRequest_429BodyFallback(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		// No Retry-After header: the wait comes from the body.
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"code":"rate_limited","message":"Workflow declaration rate limit exceeded.","retry_after_seconds":2}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	c.ThrottleRetries = 0 // will be clamped to 1
	start := time.Now()
	_, err := c.Get(context.Background(), "/test")

	var throttled *ThrottledError
	if !errors.As(err, &throttled) {
		t.Fatalf("expected ThrottledError, got %T: %v", err, err)
	}
	if throttled.RetryAfterSeconds != 2 {
		t.Errorf("RetryAfterSeconds = %d, want 2 (from body fallback)", throttled.RetryAfterSeconds)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls (one retry), got %d", got)
	}
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Errorf("retried after %s, want a wait of at least 2s", elapsed)
	}
}

func TestDoRequest_429AuthFailureNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"code":"auth_failure_rate_limited","message":"Too many authentication failures.","retry_after_seconds":38,"scope":"project_api_auth"}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	start := time.Now()
	_, err := c.Get(context.Background(), "/test")

	var throttled *ThrottledError
	if !errors.As(err, &throttled) {
		t.Fatalf("expected ThrottledError, got %T: %v", err, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 call (authentication-failure 429s are not retried), got %d", got)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("returned after %s, want no wait", elapsed)
	}
	if throttled.RetryAfterSeconds != 38 {
		t.Errorf("RetryAfterSeconds = %d, want 38", throttled.RetryAfterSeconds)
	}
	if throttled.Code != "auth_failure_rate_limited" {
		t.Errorf("Code = %q, want auth_failure_rate_limited", throttled.Code)
	}
	if !strings.Contains(err.Error(), "Too many authentication failures.") {
		t.Errorf("Error() = %q, want it to carry the API message", err.Error())
	}
}

func TestDoRequest_429WaitAboveCapNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"code":"rate_limited","message":"Too many requests"}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	_, err := c.Get(context.Background(), "/test")

	var throttled *ThrottledError
	if !errors.As(err, &throttled) {
		t.Fatalf("expected ThrottledError, got %T: %v", err, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 call (wait above %ds is not retried), got %d", MaxThrottleWaitSeconds, got)
	}
	if throttled.RetryAfterSeconds != 3600 {
		t.Errorf("RetryAfterSeconds = %d, want 3600", throttled.RetryAfterSeconds)
	}
}

func TestDoRequest_403NoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"code":"approval.authority_configuration_required","message":"Approval credentials require independently approved authority configuration."}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	_, err := c.Get(context.Background(), "/test")
	if err == nil {
		t.Fatal("expected error on 403")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
	if apiErr.Code != "approval.authority_configuration_required" {
		t.Errorf("Code = %q, want approval.authority_configuration_required", apiErr.Code)
	}
	want := "API error (status 403): approval.authority_configuration_required: Approval credentials require independently approved authority configuration."
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 call (no retry on 403), got %d", calls)
	}
}

func TestAPIErrorFormatsKnownBodyShapes(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
		field  string
	}{
		{
			name:   "validation error names the field",
			status: http.StatusBadRequest,
			body:   `{"error":{"code":"invalid_request","message":"Invalid request payload.","field":"decision"}}`,
			want:   `API error (status 400): invalid_request: Invalid request payload. (field "decision")`,
			field:  "decision",
		},
		{
			name:   "bare detail body from an unknown route",
			status: http.StatusNotFound,
			body:   `{"detail":"Not Found"}`,
			want:   "API error (status 404): Not Found",
		},
		{
			name:   "unrecognized JSON is shown verbatim",
			status: http.StatusForbidden,
			body:   `{"error":"denied"}`,
			want:   `API error (status 403): {"error":"denied"}`,
		},
		{
			name:   "non-JSON body is shown verbatim",
			status: http.StatusBadGateway,
			body:   "upstream unavailable\n",
			want:   "API error (status 502): upstream unavailable",
		},
		{
			name:   "empty body",
			status: http.StatusInternalServerError,
			body:   "",
			want:   "API error (status 500): (empty response body)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := newAPIError(tc.status, []byte(tc.body))
			if got := err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
			if err.Field != tc.field {
				t.Errorf("Field = %q, want %q", err.Field, tc.field)
			}
		})
	}
}

func TestPostWithStatusReturnsAcceptedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"pending_change_id":"chg_1"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	body, status, err := c.PostWithStatus(context.Background(), "/test", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("PostWithStatus returned error: %v", err)
	}
	if status != http.StatusAccepted {
		t.Errorf("status = %d, want 202", status)
	}
	if string(body) != `{"pending_change_id":"chg_1"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestParseRetryAfter_HeaderPreferred(t *testing.T) {
	body := []byte(`{"error":{"code":"rate_limited","message":"Too many requests","retry_after_seconds":30}}`)
	got := parseRetryAfter("5", body)
	if got != 5 {
		t.Errorf("parseRetryAfter = %d, want 5 (header takes precedence)", got)
	}
}

func TestParseRetryAfter_BodyShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"error.details", `{"error":{"code":"rate_limited","message":"m","details":{"retry_after_seconds":7}}}`, 7},
		{"error.details preferred over flattened", `{"error":{"code":"rate_limited","message":"m","retry_after_seconds":9,"details":{"retry_after_seconds":7}}}`, 7},
		{"flattened into error", `{"error":{"code":"auth_failure_rate_limited","message":"m","retry_after_seconds":38,"scope":"project_api_auth"}}`, 38},
		{"fractional seconds round up", `{"error":{"code":"rate_limited","message":"m","retry_after_seconds":2.5}}`, 3},
		{"legacy permit shape", `{"permit":{"outcome_detail":{"retry_after_seconds":30}}}`, 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRetryAfter("", []byte(tc.body)); got != tc.want {
				t.Errorf("parseRetryAfter = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseRetryAfter_Default(t *testing.T) {
	got := parseRetryAfter("", []byte(`{"error":{"code":"rate_limited","message":"Too many requests"}}`))
	if got != 1 {
		t.Errorf("parseRetryAfter = %d, want 1 (default)", got)
	}
	if got := parseRetryAfter("not-a-number", []byte(`not json`)); got != 1 {
		t.Errorf("parseRetryAfter = %d, want 1 (default)", got)
	}
}
