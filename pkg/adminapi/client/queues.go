package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListQueues(ctx context.Context) ([]models.QueueDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/queues", nil)
	if err != nil {
		return nil, err
	}

	var resp models.QueueListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Queues, nil
}

func (c *Client) GetQueue(ctx context.Context, vhost, queue string) (*models.QueueDTO, error) {
	path := "/queues/" + url.PathEscape(vhost) + "/" + url.PathEscape(queue)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.QueueDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
