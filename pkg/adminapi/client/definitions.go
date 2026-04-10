package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ottermq/ottermq/internal/core/models"
)

func (c *Client) ExportDefinitions(ctx context.Context) (*models.DefinitionsDTO, error) {
	return c.exportDefinitions(ctx, "/definitions")
}

func (c *Client) ExportVHostDefinitions(ctx context.Context, vhost string) (*models.DefinitionsDTO, error) {
	return c.exportDefinitions(ctx, fmt.Sprintf("/definitions/%s", url.PathEscape(vhost)))
}

func (c *Client) ImportDefinitions(ctx context.Context, defs *models.DefinitionsDTO) (*models.DefinitionsImportResponse, error) {
	return c.importDefinitions(ctx, "/definitions", defs)
}

func (c *Client) ImportVHostDefinitions(ctx context.Context, vhost string, defs *models.DefinitionsDTO) (*models.DefinitionsImportResponse, error) {
	return c.importDefinitions(ctx, fmt.Sprintf("/definitions/%s", url.PathEscape(vhost)), defs)
}

func (c *Client) exportDefinitions(ctx context.Context, path string) (*models.DefinitionsDTO, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var out models.DefinitionsDTO
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) importDefinitions(ctx context.Context, path string, defs *models.DefinitionsDTO) (*models.DefinitionsImportResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, path, defs)
	if err != nil {
		return nil, err
	}
	var out models.DefinitionsImportResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
