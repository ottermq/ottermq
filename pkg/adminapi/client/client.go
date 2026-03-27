package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ottermq/ottermq/internal/core/models"
)

const (
	DefaultBaseURL = "http://localhost:3000"
	DefaultAPIPrefix = "/api"
)

type Client struct {
	baseURL    string
	apiPrefix  string
	token      string
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("api request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("api request failed with status %d: %s", e.StatusCode, e.Message)
}

func New(baseURL string, httpClient *http.Client) *Client {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiPrefix:  DefaultAPIPrefix,
		httpClient: httpClient,
	}
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) APIPrefix() string {
	return c.apiPrefix
}

func (c *Client) Token() string {
	return c.token
}

func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+c.apiPrefix+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(resp)
	}

	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func decodeAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{StatusCode: resp.StatusCode, Message: resp.Status}
	}

	var errResp models.ErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var unauthorized models.UnauthorizedErrorResponse
	if json.Unmarshal(body, &unauthorized) == nil && unauthorized.Error != "" {
		return &APIError{StatusCode: resp.StatusCode, Message: unauthorized.Error}
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}

	return &APIError{StatusCode: resp.StatusCode, Message: message}
}
