package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
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
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
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
