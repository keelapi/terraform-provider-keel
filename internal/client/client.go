package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MaxThrottleRetries is the hard cap for 429 retry attempts.
const MaxThrottleRetries = 3

// MaxThrottleWaitSeconds caps how long the client waits before retrying a 429.
// When Keel asks for a longer wait, the ThrottledError is returned at once
// rather than blocking the Terraform run.
const MaxThrottleWaitSeconds = 60

// Client calls the Keel API with a single bearer credential.
type Client struct {
	BaseURL string
	// Token is sent as "Authorization: Bearer <Token>": a Keel API key, or a
	// Keel user access token for the routes that accept only a user.
	Token           string
	HTTPClient      *http.Client
	ThrottleRetries int // 0 means use default (1). Hard-capped at MaxThrottleRetries.
}

// ProviderData carries the provider's Keel credentials to resources and data
// sources. A field is nil when its credential is not configured.
type ProviderData struct {
	// APIKey authenticates with the provider's Keel API key (api_key or
	// KEEL_API_KEY). keel_api_key and keel_permit use it.
	APIKey *Client
	// UserToken authenticates with a Keel user access token (user_token or
	// KEEL_USER_TOKEN). Only keel_organization_member uses it: Keel's
	// organization member routes do not accept API keys.
	UserToken *Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL:         baseURL,
		Token:           token,
		ThrottleRetries: 1,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) maxRetries() int {
	n := c.ThrottleRetries
	if n <= 0 {
		n = 1
	}
	if n > MaxThrottleRetries {
		n = MaxThrottleRetries
	}
	return n
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling request body: %w", err)
		}
	}

	maxAttempts := 1 + c.maxRetries() // first attempt + retries
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		reqURL := fmt.Sprintf("%s%s", c.BaseURL, path)
		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("executing request: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
		}

		// Handle 429 throttle with retry.
		if resp.StatusCode == http.StatusTooManyRequests {
			throttled := newThrottledError(resp.Header.Get("Retry-After"), respBody)

			if attempt < maxAttempts-1 && throttled.retryable() {
				wait := time.Duration(throttled.RetryAfterSeconds) * time.Second
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return nil, resp.StatusCode, ctx.Err()
				}
			}

			// Not retryable, or retries exhausted.
			return respBody, resp.StatusCode, throttled
		}

		if resp.StatusCode >= 400 {
			return respBody, resp.StatusCode, newAPIError(resp.StatusCode, respBody)
		}

		return respBody, resp.StatusCode, nil
	}

	// Unreachable, but satisfy the compiler.
	return nil, 0, fmt.Errorf("unexpected retry loop exit")
}

// parseRetryAfter extracts the retry delay in seconds. It prefers the
// Retry-After header; if absent or unparseable it falls back to the
// retry_after_seconds value in the response body (see
// errorEnvelope.retryAfterSeconds). Returns 1 as a minimum.
func parseRetryAfter(header string, body []byte) int {
	if header != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs > 0 {
			return secs
		}
	}
	if secs := parseErrorEnvelope(body).retryAfterSeconds(); secs > 0 {
		return secs
	}
	return 1
}

func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	data, _, err := c.doRequest(ctx, http.MethodGet, path, nil)
	return data, err
}

func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, error) {
	data, _, err := c.doRequest(ctx, http.MethodPost, path, body)
	return data, err
}

// PostWithStatus is Post, also returning the HTTP status code so callers can
// tell a 201 from a 202 (accepted, pending approval).
func (c *Client) PostWithStatus(ctx context.Context, path string, body any) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

func (c *Client) Put(ctx context.Context, path string, body any) ([]byte, error) {
	data, _, err := c.doRequest(ctx, http.MethodPut, path, body)
	return data, err
}

func (c *Client) Patch(ctx context.Context, path string, body any) ([]byte, error) {
	data, _, err := c.doRequest(ctx, http.MethodPatch, path, body)
	return data, err
}

func (c *Client) Delete(ctx context.Context, path string) error {
	_, _, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

// GetWithStatus is Get, also returning the HTTP status code.
func (c *Client) GetWithStatus(ctx context.Context, path string) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}
