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

func (c *Client) CreateExchange(ctx context.Context, vhost, exchange string, reqBody models.CreateExchangeRequest) (*models.SuccessResponse, error) {
	path := "/exchanges/" + url.PathEscape(vhost) + "/" + url.PathEscape(exchange)
	req, err := c.newRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var resp models.SuccessResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) DeleteExchange(ctx context.Context, vhost, exchange string) error {
	path := "/exchanges/" + url.PathEscape(vhost) + "/" + url.PathEscape(exchange)
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

func (c *Client) PublishMessage(ctx context.Context, vhost, exchange string, reqBody models.PublishMessageRequest) (*models.SuccessResponse, error) {
	path := "/exchanges/" + url.PathEscape(vhost) + "/" + url.PathEscape(exchange) + "/publish"
	req, err := c.newRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var resp models.SuccessResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
