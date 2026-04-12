package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDoRequest_429Retry_Success(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"permit":{"decision":"throttled","reason_code":"budget.rate_limit_throttled","outcome_detail":{"retry_after_seconds":1}}}`)
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
		fmt.Fprint(w, `{"permit":{"permit_id":"pmt_123","decision":"throttled","reason_code":"budget.rate_limit_throttled","outcome_detail":{"retry_after_seconds":1}}}`)
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
	if throttled.PermitID != "pmt_123" {
		t.Errorf("PermitID = %q, want pmt_123", throttled.PermitID)
	}
	if throttled.ReasonCode != "budget.rate_limit_throttled" {
		t.Errorf("ReasonCode = %q, want budget.rate_limit_throttled", throttled.ReasonCode)
	}
}

func TestDoRequest_403NoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"denied"}`)
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
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 call (no retry on 403), got %d", calls)
	}
}

func TestDoRequest_429BodyFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Retry-After header — should fall back to body.
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"permit":{"decision":"throttled","reason_code":"budget.rate_limit_throttled","outcome_detail":{"retry_after_seconds":2}}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	c.ThrottleRetries = 0 // will be clamped to 1
	_, err := c.Get(context.Background(), "/test")

	var throttled *ThrottledError
	if !errors.As(err, &throttled) {
		t.Fatalf("expected ThrottledError, got %T: %v", err, err)
	}
	if throttled.RetryAfterSeconds != 2 {
		t.Errorf("RetryAfterSeconds = %d, want 2 (from body fallback)", throttled.RetryAfterSeconds)
	}
}

func TestParseRetryAfter_HeaderPreferred(t *testing.T) {
	body := []byte(`{"permit":{"outcome_detail":{"retry_after_seconds":30}}}`)
	got := parseRetryAfter("5", body)
	if got != 5 {
		t.Errorf("parseRetryAfter = %d, want 5 (header takes precedence)", got)
	}
}

func TestParseRetryAfter_BodyFallback(t *testing.T) {
	body := []byte(`{"permit":{"outcome_detail":{"retry_after_seconds":30}}}`)
	got := parseRetryAfter("", body)
	if got != 30 {
		t.Errorf("parseRetryAfter = %d, want 30 (body fallback)", got)
	}
}

func TestParseRetryAfter_Default(t *testing.T) {
	got := parseRetryAfter("", []byte(`{}`))
	if got != 1 {
		t.Errorf("parseRetryAfter = %d, want 1 (default)", got)
	}
}
