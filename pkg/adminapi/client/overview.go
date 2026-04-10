package client

import (
	"context"
	"net/http"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) GetOverview(ctx context.Context) (*models.OverviewDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/overview", nil)
	if err != nil {
		return nil, err
	}

	var resp models.OverviewDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
