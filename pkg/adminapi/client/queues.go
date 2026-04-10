package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

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

func (c *Client) CreateQueue(ctx context.Context, vhost, queue string, reqBody models.CreateQueueRequest) (*models.SuccessResponse, error) {
	path := "/queues/" + url.PathEscape(vhost) + "/"
	if queue != "" {
		path += url.PathEscape(queue)
	}

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

func (c *Client) DeleteQueue(ctx context.Context, vhost, queue string, ifUnused, ifEmpty bool) error {
	path := "/queues/" + url.PathEscape(vhost) + "/" + url.PathEscape(queue)
	values := url.Values{}
	if ifUnused {
		values.Set("ifUnused", strconv.FormatBool(true))
	}
	if ifEmpty {
		values.Set("ifEmpty", strconv.FormatBool(true))
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

func (c *Client) PurgeQueue(ctx context.Context, vhost, queue string) error {
	path := "/queues/" + url.PathEscape(vhost) + "/" + url.PathEscape(queue) + "/content"
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

func (c *Client) GetMessages(ctx context.Context, vhost, queue string, reqBody models.GetMessageRequest) ([]models.MessageDTO, error) {
	path := "/queues/" + url.PathEscape(vhost) + "/" + url.PathEscape(queue) + "/get"
	req, err := c.newRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var resp models.MessageListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Messages, nil
}
