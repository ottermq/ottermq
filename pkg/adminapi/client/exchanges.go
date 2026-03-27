package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListExchanges(ctx context.Context) ([]models.ExchangeDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/exchanges", nil)
	if err != nil {
		return nil, err
	}

	var resp models.ExchangeListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Exchanges, nil
}

func (c *Client) GetExchange(ctx context.Context, vhost, exchange string) (*models.ExchangeDTO, error) {
	path := "/exchanges/" + url.PathEscape(vhost) + "/" + url.PathEscape(exchange)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.ExchangeDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
