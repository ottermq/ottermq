package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type opsRoundTripper func(*http.Request) (*http.Response, error)

func (f opsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func opsJSONResponse(t *testing.T, status int, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

func newOpsRuntime(opts *RootOptions, fn opsRoundTripper, stdout *bytes.Buffer) *Runtime {
	return &Runtime{
		Options: opts,
		HTTPClient: &http.Client{
			Transport: fn,
		},
		Stdout: stdout,
	}
}

func TestConnectionsListCommand_TextOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := newOpsRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/connections", req.URL.String())
		return opsJSONResponse(t, http.StatusOK, models.ConnectionListResponse{
			Connections: []models.ConnectionInfoDTO{{Name: "127.0.0.1:5672", VHostName: "/", Username: "guest", State: "running", Channels: 2}},
		}), nil
	}, &stdout)

	cmd := NewConnectionsCmd(rt)
	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "127.0.0.1:5672")
	assert.Contains(t, stdout.String(), "channels=2")
}

func TestConnectionsCloseCommand_TextOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := newOpsRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/connections/127.0.0.1:5672?reason=bye", req.URL.String())
		return opsJSONResponse(t, http.StatusOK, models.SuccessResponse{Message: "Connection closed"}), nil
	}, &stdout)

	cmd := NewConnectionsCmd(rt)
	cmd.SetArgs([]string{"close", "127.0.0.1:5672", "--reason", "bye"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "Connection closed\n", stdout.String())
}

func TestChannelsListCommand_UsesConnectionFilter(t *testing.T) {
	var stdout bytes.Buffer
	rt := newOpsRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/connections/conn-1/channels", req.URL.String())
		return opsJSONResponse(t, http.StatusOK, models.ChannelListResponse{
			Channels: []models.ChannelDetailDTO{{ConnectionName: "conn-1", Number: 1, VHost: "/", User: "guest", State: "running"}},
		}), nil
	}, &stdout)

	cmd := NewChannelsCmd(rt)
	cmd.SetArgs([]string{"list", "--connection", "conn-1"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "conn-1#1")
}

func TestChannelsGetCommand_RejectsInvalidChannel(t *testing.T) {
	rt := newOpsRuntime(&RootOptions{Token: "jwt-token"}, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected http request")
		return nil, nil
	}, &bytes.Buffer{})

	cmd := NewChannelsCmd(rt)
	cmd.SetArgs([]string{"get", "conn-1", "abc"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel must be a valid integer")
}

func TestConsumersListCommand_RequiresVHostForQueueFilter(t *testing.T) {
	rt := newOpsRuntime(&RootOptions{Token: "jwt-token"}, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected http request")
		return nil, nil
	}, &bytes.Buffer{})

	cmd := NewConsumersCmd(rt)
	cmd.SetArgs([]string{"list", "--queue", "jobs"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vhost is required when filtering by queue")
}

func TestConsumersListCommand_UsesQueueScopedRoute(t *testing.T) {
	var stdout bytes.Buffer
	rt := newOpsRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/queues/%2F/jobs/consumers", req.URL.String())
		return opsJSONResponse(t, http.StatusOK, models.ConsumerListResponse{
			Consumers: []models.ConsumerDTO{{ConsumerTag: "ctag-1", QueueName: "jobs", Active: true}},
		}), nil
	}, &stdout)

	cmd := NewConsumersCmd(rt)
	cmd.SetArgs([]string{"list", "--vhost", "/", "--queue", "jobs"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "ctag-1")
	assert.Contains(t, stdout.String(), "queue=jobs")
}
