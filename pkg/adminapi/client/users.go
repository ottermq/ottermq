package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ListUsers(ctx context.Context) ([]models.UserSummary, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/admin/users", nil)
	if err != nil {
		return nil, err
	}
	var resp models.UserListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Users, nil
}

func (c *Client) GetUser(ctx context.Context, username string) (*models.UserSummary, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/admin/users/"+url.PathEscape(username), nil)
	if err != nil {
		return nil, err
	}
	var resp models.UserSummary
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateUser(ctx context.Context, username, password string, roleID int) (*models.SuccessResponse, error) {
	body := models.UserCreateRequest{
		Username:        username,
		Password:        password,
		ConfirmPassword: password,
		Role:            roleID,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/admin/users", body)
	if err != nil {
		return nil, err
	}
	var resp models.SuccessResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteUser(ctx context.Context, username string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/admin/users/"+url.PathEscape(username), nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) ChangePassword(ctx context.Context, username, password string) (*models.SuccessResponse, error) {
	body := models.UserPasswordUpdateRequest{
		Password:        password,
		ConfirmPassword: password,
	}
	req, err := c.newRequest(ctx, http.MethodPut, fmt.Sprintf("/admin/users/%s/password", url.PathEscape(username)), body)
	if err != nil {
		return nil, err
	}
	var resp models.SuccessResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
