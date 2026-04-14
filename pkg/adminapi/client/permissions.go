package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListPermissions(ctx context.Context) ([]models.PermissionDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/admin/permissions", nil)
	if err != nil {
		return nil, err
	}
	var resp models.PermissionListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Permissions, nil
}

func (c *Client) GetPermission(ctx context.Context, vhost, username string) (*models.PermissionDTO, error) {
	path := fmt.Sprintf("/admin/permissions/%s/%s", url.PathEscape(vhost), url.PathEscape(username))
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp models.PermissionDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GrantPermission(ctx context.Context, vhost, username string) (*models.PermissionDTO, error) {
	path := fmt.Sprintf("/admin/permissions/%s/%s", url.PathEscape(vhost), url.PathEscape(username))
	req, err := c.newRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return nil, err
	}
	var resp models.PermissionDTO
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RevokePermission(ctx context.Context, vhost, username string) error {
	path := fmt.Sprintf("/admin/permissions/%s/%s", url.PathEscape(vhost), url.PathEscape(username))
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}
