package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListNodes(ctx context.Context) ([]models.OverviewNodeDetails, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/nodes", nil)
	if err != nil {
		return nil, err
	}
	var out []models.OverviewNodeDetails
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetNode(ctx context.Context, name string) (*models.OverviewNodeDetails, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s", url.PathEscape(name)), nil)
	if err != nil {
		return nil, err
	}
	var out models.OverviewNodeDetails
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetNodeMemory(ctx context.Context, name string) (*models.NodeMemoryDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/memory", url.PathEscape(name)), nil)
	if err != nil {
		return nil, err
	}
	var out models.NodeMemoryDTO
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
