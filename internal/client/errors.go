package client

import "fmt"

// APIError represents a non-2xx response from the Keel API.
type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, string(e.Body))
}

// ThrottledError is returned when the API responds with HTTP 429 and all
// retries have been exhausted.
type ThrottledError struct {
	RetryAfterSeconds int
	PermitID          string
	ReasonCode        string
	Body              []byte
}

func (e *ThrottledError) Error() string {
	return fmt.Sprintf("rate-limit throttled (retry_after=%ds, reason=%s)", e.RetryAfterSeconds, e.ReasonCode)
}
