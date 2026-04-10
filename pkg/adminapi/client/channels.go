package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListChannels(ctx context.Context, vhost, connection string) ([]models.ChannelDetailDTO, error) {
	path := "/channels"
	if connection != "" {
		path = "/connections/" + url.PathEscape(connection) + "/channels"
	} else if vhost != "" {
		path += "/" + url.PathEscape(vhost)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.ChannelListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Channels, nil
}

func (c *Client) GetChannel(ctx context.Context, connection string, number uint16) (*models.ChannelDetailDTO, error) {
	path := "/connections/" + url.PathEscape(connection) + "/channels/" + strconv.FormatUint(uint64(number), 10)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.ChannelDetailDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
