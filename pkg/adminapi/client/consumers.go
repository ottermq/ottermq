package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListConsumers(ctx context.Context, vhost, queue string) ([]models.ConsumerDTO, error) {
	path := "/consumers"
	switch {
	case vhost != "" && queue != "":
		path = "/queues/" + url.PathEscape(vhost) + "/" + url.PathEscape(queue) + "/consumers"
	case vhost != "":
		path += "/" + url.PathEscape(vhost)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.ConsumerListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Consumers, nil
}
