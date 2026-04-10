package client

import (
	"context"
	"net/http"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) Login(ctx context.Context, username, password string) (*models.AuthResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/login", models.AuthRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, err
	}

	var resp models.AuthResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	if resp.Token != "" {
		c.SetToken(resp.Token)
	}

	return &resp, nil
}
