package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loginRoundTripper func(*http.Request) (*http.Response, error)

func (f loginRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func loginTestResponse(t *testing.T, req *http.Request, unauthorized bool) (*http.Response, error) {
	t.Helper()

	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "http://example.test/api/login", req.URL.String())

	var authReq models.AuthRequest
	require.NoError(t, json.NewDecoder(req.Body).Decode(&authReq))

	if unauthorized {
		return jsonHTTPResponse(t, http.StatusUnauthorized, models.ErrorResponse{
			Error: "invalid credentials",
		}), nil
	}

	assert.Equal(t, "guest", authReq.Username)
	assert.NotEmpty(t, authReq.Password)

	return jsonHTTPResponse(t, http.StatusOK, models.AuthResponse{
		Token: "jwt-token",
	}), nil
}

func jsonHTTPResponse(t *testing.T, status int, body any) *http.Response {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

func TestLoginCommand_RequiresUsername(t *testing.T) {
	rt := NewRuntime(&RootOptions{Password: "secret"})
	cmd := NewLoginCmd(rt)

	err := cmd.Execute()
	require.Error(t, err)
	assert.EqualError(t, err, "username is required")
}

func TestLoginCommand_RequiresPassword(t *testing.T) {
	rt := NewRuntime(&RootOptions{Username: "guest"})
	cmd := NewLoginCmd(rt)

	err := cmd.Execute()
	require.Error(t, err)
	assert.EqualError(t, err, "password is required")
}

func TestLoginCommand_StoresTokenAndPrintsSuccess(t *testing.T) {
	var stdout bytes.Buffer
	rt := &Runtime{
		Options: &RootOptions{
			BaseURL:  "http://example.test",
			Username: "guest",
			Password: "guest",
		},
		HTTPClient: &http.Client{
			Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				return loginTestResponse(t, req, false)
			}),
		},
		Stdout: &stdout,
	}

	cmd := NewLoginCmd(rt)
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "jwt-token", rt.Options.Token)
	assert.Contains(t, stdout.String(), "Login successful")
}

func TestLoginCommand_JSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := &Runtime{
		Options: &RootOptions{
			BaseURL:  "http://example.test",
			Username: "guest",
			Password: "guest",
			JSON:     true,
		},
		HTTPClient: &http.Client{
			Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				return loginTestResponse(t, req, false)
			}),
		},
		Stdout: &stdout,
	}

	cmd := NewLoginCmd(rt)
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), `"token": "jwt-token"`)
}

func TestLoginCommand_PropagatesClientError(t *testing.T) {
	rt := &Runtime{
		Options: &RootOptions{
			BaseURL:  "http://example.test",
			Username: "guest",
			Password: "wrong",
		},
		HTTPClient: &http.Client{
			Transport: loginRoundTripper(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("network unavailable")
			}),
		},
		Stdout: &bytes.Buffer{},
	}

	cmd := NewLoginCmd(rt)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send request")
}

func TestRuntimeClient_UsesTokenFromOptions(t *testing.T) {
	rt := NewRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	})

	client := rt.Client()
	assert.Equal(t, "jwt-token", client.Token())
	assert.Equal(t, "http://example.test", client.BaseURL())
}

func TestRuntimeContext_NotNil(t *testing.T) {
	rt := NewRuntime(&RootOptions{})
	assert.Equal(t, context.Background(), rt.Context())
}
