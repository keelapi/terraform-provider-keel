package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// MaxThrottleRetries is the hard cap for 429 retry attempts.
const MaxThrottleRetries = 3

type Client struct {
	BaseURL        string
	APIKey         string
	HTTPClient     *http.Client
	ThrottleRetries int // 0 means use default (1). Hard-capped at MaxThrottleRetries.
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL:         baseURL,
		APIKey:          apiKey,
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

		req.Header.Set("Authorization", "Bearer "+c.APIKey)
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
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), respBody)

			if attempt < maxAttempts-1 {
				wait := time.Duration(retryAfter) * time.Second
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return nil, resp.StatusCode, ctx.Err()
				}
			}

			// Retries exhausted — return ThrottledError.
			permitID, reasonCode := parseThrottleBody(respBody)
			return respBody, resp.StatusCode, &ThrottledError{
				RetryAfterSeconds: retryAfter,
				PermitID:          permitID,
				ReasonCode:        reasonCode,
				Body:              respBody,
			}
		}

		if resp.StatusCode >= 400 {
			return respBody, resp.StatusCode, &APIError{
				StatusCode: resp.StatusCode,
				Body:       respBody,
			}
		}

		return respBody, resp.StatusCode, nil
	}

	// Unreachable, but satisfy the compiler.
	return nil, 0, fmt.Errorf("unexpected retry loop exit")
}

// parseRetryAfter extracts the retry delay in seconds. It prefers the
// Retry-After header; if absent or unparseable it falls back to the
// retry_after_seconds value in the response body. Returns 1 as a minimum.
func parseRetryAfter(header string, body []byte) int {
	if header != "" {
		if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
			return secs
		}
	}
	var envelope struct {
		Permit struct {
			OutcomeDetail struct {
				RetryAfterSeconds int `json:"retry_after_seconds"`
			} `json:"outcome_detail"`
		} `json:"permit"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Permit.OutcomeDetail.RetryAfterSeconds > 0 {
		return envelope.Permit.OutcomeDetail.RetryAfterSeconds
	}
	return 1
}

// parseThrottleBody extracts permit_id and reason_code from a 429 body.
func parseThrottleBody(body []byte) (permitID, reasonCode string) {
	var envelope struct {
		Permit struct {
			PermitID   string `json:"permit_id"`
			ReasonCode string `json:"reason_code"`
		} `json:"permit"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		return envelope.Permit.PermitID, envelope.Permit.ReasonCode
	}
	return "", ""
}

func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	data, _, err := c.doRequest(ctx, http.MethodGet, path, nil)
	return data, err
}

func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, error) {
	data, _, err := c.doRequest(ctx, http.MethodPost, path, body)
	return data, err
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

// IsNotFound returns true if the error represents a 404 response.
func (c *Client) GetWithStatus(ctx context.Context, path string) ([]byte, int, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}
