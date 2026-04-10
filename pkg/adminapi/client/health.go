package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) CheckAlarms(ctx context.Context) (*models.HealthCheckResponse, error) {
	return c.doHealthCheck(ctx, "/health/checks/alarms")
}

func (c *Client) CheckLocalAlarms(ctx context.Context) (*models.HealthCheckResponse, error) {
	return c.doHealthCheck(ctx, "/health/checks/local-alarms")
}

func (c *Client) CheckPortListener(ctx context.Context, port string) (*models.HealthCheckResponse, error) {
	return c.doHealthCheck(ctx, fmt.Sprintf("/health/checks/port-listener/%s", port))
}

func (c *Client) CheckVirtualHosts(ctx context.Context) (*models.HealthCheckResponse, error) {
	return c.doHealthCheck(ctx, "/health/checks/virtual-hosts")
}

func (c *Client) CheckReady(ctx context.Context) (*models.HealthCheckResponse, error) {
	return c.doHealthCheck(ctx, "/health/checks/ready")
}

// doHealthCheck performs a health check request. It tolerates 503 responses
// (unhealthy but reachable) and returns the decoded body rather than an error.
func (c *Client) doHealthCheck(ctx context.Context, path string) (*models.HealthCheckResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	var out models.HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil && err != io.EOF {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}
