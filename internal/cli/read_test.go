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

type readRoundTripper func(*http.Request) (*http.Response, error)

func (f readRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func readJSONResponse(t *testing.T, status int, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

func newReadRuntime(opts *RootOptions, fn readRoundTripper, stdout *bytes.Buffer) *Runtime {
	return &Runtime{
		Options: opts,
		HTTPClient: &http.Client{
			Transport: fn,
		},
		Stdout: stdout,
	}
}

func TestOverviewCommand_TextOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := newReadRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "http://example.test/api/overview", req.URL.String())
		require.Equal(t, "Bearer jwt-token", req.Header.Get("Authorization"))
		return readJSONResponse(t, http.StatusOK, models.OverviewDTO{
			BrokerDetails: models.OverviewBrokerDetails{Product: "OtterMQ", Version: "1.0.0"},
			ObjectTotals:  models.OverviewObjectTotals{Connections: 2, Channels: 4, Queues: 6, Consumers: 3},
			MessageStats:  models.OverviewMessageStats{MessagesReady: 8, MessagesUnacked: 1},
		}), nil
	}, &stdout)

	cmd := NewOverviewCmd(rt)
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Broker: OtterMQ 1.0.0")
	assert.Contains(t, stdout.String(), "Queues: 6")
}

func TestQueuesListCommand_JSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := newReadRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
		JSON:    true,
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/queues", req.URL.String())
		return readJSONResponse(t, http.StatusOK, models.QueueListResponse{
			Queues: []models.QueueDTO{{VHost: "/", Name: "jobs", Messages: 3}},
		}), nil
	}, &stdout)

	cmd := NewQueuesCmd(rt)
	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "jobs"`)
}

func TestQueuesGetCommand_RequiresArgs(t *testing.T) {
	rt := newReadRuntime(&RootOptions{Token: "jwt-token"}, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected http request")
		return nil, nil
	}, &bytes.Buffer{})

	cmd := NewQueuesCmd(rt)
	cmd.SetArgs([]string{"get"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg(s)")
}

func TestExchangesGetCommand_TextOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := newReadRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/exchanges/%2F/amq.direct", req.URL.String())
		return readJSONResponse(t, http.StatusOK, models.ExchangeDTO{
			VHost: "/", Name: "amq.direct", Type: "direct", Durable: true,
		}), nil
	}, &stdout)

	cmd := NewExchangesCmd(rt)
	cmd.SetArgs([]string{"get", "/", "amq.direct"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Exchange: //amq.direct")
	assert.Contains(t, stdout.String(), "Type: direct")
}

func TestBindingsListCommand_UsesVHostFlag(t *testing.T) {
	var stdout bytes.Buffer
	rt := newReadRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/bindings/%2F", req.URL.String())
		return readJSONResponse(t, http.StatusOK, models.BindingListResponse{
			Bindings: []models.BindingDTO{{VHost: "/", Source: "amq.direct", Destination: "jobs", RoutingKey: "jobs"}},
		}), nil
	}, &stdout)

	cmd := NewBindingsCmd(rt)
	cmd.SetArgs([]string{"list", "--vhost", "/"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "amq.direct -> jobs")
}

func TestReadCommands_LoginWithUsernamePasswordWhenTokenMissing(t *testing.T) {
	var stdout bytes.Buffer
	callCount := 0
	rt := newReadRuntime(&RootOptions{
		BaseURL:  "http://example.test",
		Username: "guest",
		Password: "guest",
	}, func(req *http.Request) (*http.Response, error) {
		callCount++
		switch req.URL.String() {
		case "http://example.test/api/login":
			return readJSONResponse(t, http.StatusOK, models.AuthResponse{Token: "jwt-token"}), nil
		case "http://example.test/api/queues":
			require.Equal(t, "Bearer jwt-token", req.Header.Get("Authorization"))
			return readJSONResponse(t, http.StatusOK, models.QueueListResponse{
				Queues: []models.QueueDTO{{VHost: "/", Name: "jobs"}},
			}), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}, &stdout)

	cmd := NewQueuesCmd(rt)
	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "jwt-token", rt.Options.Token)
	assert.GreaterOrEqual(t, callCount, 2)
}
