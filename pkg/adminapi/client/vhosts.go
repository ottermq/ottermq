package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListVHosts(ctx context.Context) ([]models.VHostDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/vhosts", nil)
	if err != nil {
		return nil, err
	}
	var resp models.VHostListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.VHosts, nil
}

func (c *Client) GetVHost(ctx context.Context, name string) (*models.VHostDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/vhosts/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	var resp models.VHostDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateVHost(ctx context.Context, name string) (*models.VHostDTO, error) {
	req, err := c.newRequest(ctx, http.MethodPut, "/vhosts/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	var resp models.VHostDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteVHost(ctx context.Context, name string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/vhosts/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}
