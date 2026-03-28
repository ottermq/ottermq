package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListConnections(ctx context.Context) ([]models.ConnectionInfoDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/connections", nil)
	if err != nil {
		return nil, err
	}

	var resp models.ConnectionListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Connections, nil
}

func (c *Client) GetConnection(ctx context.Context, name string) (*models.ConnectionInfoDTO, error) {
	path := "/connections/" + url.PathEscape(name)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.ConnectionInfoDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) CloseConnection(ctx context.Context, name, reason string) (*models.SuccessResponse, error) {
	path := "/connections/" + url.PathEscape(name)
	if reason != "" {
		path += "?reason=" + url.QueryEscape(reason)
	}

	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.SuccessResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
