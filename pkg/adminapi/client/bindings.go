package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListBindings(ctx context.Context, vhost string) ([]models.BindingDTO, error) {
	path := "/bindings"
	if vhost != "" {
		path += "/" + url.PathEscape(vhost)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp models.BindingListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Bindings, nil
}
