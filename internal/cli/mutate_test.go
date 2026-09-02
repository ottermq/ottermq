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

type mutateRoundTripper func(*http.Request) (*http.Response, error)

func (f mutateRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func mutateJSONResponse(t *testing.T, status int, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

func mutateEmptyResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}
}

func newMutateRuntime(opts *RootOptions, fn mutateRoundTripper, stdout *bytes.Buffer) *Runtime {
	return &Runtime{
		Options: opts,
		HTTPClient: &http.Client{
			Transport: fn,
		},
		Stdout: stdout,
	}
}

func TestQueuesCreateCommand_CreatesGeneratedQueue(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "http://example.test/api/queues/%2F/", req.URL.String())

		var payload models.CreateQueueRequest
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		assert.True(t, payload.Durable)
		assert.True(t, payload.AutoDelete)

		return mutateJSONResponse(t, http.StatusOK, models.SuccessResponse{
			Message: "Queue 'amq.gen-1' created successfully",
		}), nil
	}, &stdout)

	cmd := NewQueuesCmd(rt)
	cmd.SetArgs([]string{"create", "/", "--durable", "--auto-delete"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "Queue 'amq.gen-1' created successfully\n", stdout.String())
}

func TestQueuesDeleteCommand_UsesConditionFlags(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, req.Method)
		require.Equal(t, "http://example.test/api/queues/%2F/jobs?ifEmpty=true&ifUnused=true", req.URL.String())
		return mutateEmptyResponse(http.StatusNoContent), nil
	}, &stdout)

	cmd := NewQueuesCmd(rt)
	cmd.SetArgs([]string{"delete", "/", "jobs", "--if-unused", "--if-empty"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "Queue deleted\n", stdout.String())
}

func TestBindingsCreateCommand_RequiresSource(t *testing.T) {
	rt := newMutateRuntime(&RootOptions{Token: "jwt-token"}, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected http request")
		return nil, nil
	}, &bytes.Buffer{})

	cmd := NewBindingsCmd(rt)
	cmd.SetArgs([]string{"create", "--destination", "jobs"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source is required")
}

func TestBindingsCreateCommand_SendsBindingRequest(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "http://example.test/api/bindings", req.URL.String())

		var payload models.CreateBindingRequest
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		assert.Equal(t, "/", payload.VHost)
		assert.Equal(t, "amq.direct", payload.Source)
		assert.Equal(t, "jobs", payload.Destination)
		assert.Equal(t, "jobs", payload.RoutingKey)

		return mutateJSONResponse(t, http.StatusOK, models.SuccessResponse{
			Message: "Queue bound to exchange",
		}), nil
	}, &stdout)

	cmd := NewBindingsCmd(rt)
	cmd.SetArgs([]string{"create", "--source", "amq.direct", "--destination", "jobs", "--routing-key", "jobs"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "Queue bound to exchange\n", stdout.String())
}

func TestPublishCommand_RequiresBody(t *testing.T) {
	rt := newMutateRuntime(&RootOptions{Token: "jwt-token"}, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected http request")
		return nil, nil
	}, &bytes.Buffer{})

	cmd := NewPublishCmd(rt)
	cmd.SetArgs([]string{"/", "events"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body is required")
}

func TestPublishCommand_PublishesMessage(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "http://example.test/api/exchanges/%2F/events/publish", req.URL.String())

		var payload models.PublishMessageRequest
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		assert.Equal(t, "jobs.created", payload.RoutingKey)
		assert.Equal(t, "hello", payload.Payload)
		assert.Equal(t, "text/plain", payload.ContentType)

		return mutateJSONResponse(t, http.StatusOK, models.SuccessResponse{
			Message: "Message published",
		}), nil
	}, &stdout)

	cmd := NewPublishCmd(rt)
	cmd.SetArgs([]string{"/", "events", "--routing-key", "jobs.created", "--body", "hello", "--content-type", "text/plain"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "Message published\n", stdout.String())
}

func TestVHostsCreateCommand_SendsRequest(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPut, req.Method)
		require.Equal(t, "http://example.test/api/vhosts/staging", req.URL.String())
		return mutateJSONResponse(t, http.StatusCreated, models.VHostDTO{Name: "staging", State: "running"}), nil
	}, &stdout)

	cmd := NewVHostsCmd(rt)
	cmd.SetArgs([]string{"create", "staging"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "staging")
}

func TestVHostsDeleteCommand_SendsRequest(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, req.Method)
		require.Equal(t, "http://example.test/api/vhosts/staging", req.URL.String())
		return mutateEmptyResponse(http.StatusNoContent), nil
	}, &stdout)

	cmd := NewVHostsCmd(rt)
	cmd.SetArgs([]string{"delete", "staging"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "staging")
}

func TestUsersCreateCommand_InvalidRole(t *testing.T) {
	rt := newMutateRuntime(&RootOptions{Token: "jwt-token"}, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected http request")
		return nil, nil
	}, &bytes.Buffer{})

	cmd := NewUsersCmd(rt)
	cmd.SetArgs([]string{"create", "alice", "--password", "s3cr3t", "--role", "superuser"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestUsersCreateCommand_SendsRequest(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "http://example.test/api/admin/users", req.URL.String())

		var payload models.UserCreateRequest
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		assert.Equal(t, "alice", payload.Username)
		assert.Equal(t, "s3cr3t", payload.Password)
		assert.Equal(t, "s3cr3t", payload.ConfirmPassword)
		assert.Equal(t, 1, payload.Role) // admin

		return mutateJSONResponse(t, http.StatusOK, models.SuccessResponse{Message: "User added successfully"}), nil
	}, &stdout)

	cmd := NewUsersCmd(rt)
	cmd.SetArgs([]string{"create", "alice", "--password", "s3cr3t", "--role", "admin"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "User added successfully\n", stdout.String())
}

func TestUsersDeleteCommand_SendsRequest(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, req.Method)
		require.Equal(t, "http://example.test/api/admin/users/alice", req.URL.String())
		return mutateEmptyResponse(http.StatusNoContent), nil
	}, &stdout)

	cmd := NewUsersCmd(rt)
	cmd.SetArgs([]string{"delete", "alice"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "alice")
}

func TestPermissionsGrantCommand_SendsRequest(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPut, req.Method)
		require.Equal(t, "http://example.test/api/admin/permissions/staging/alice", req.URL.String())
		return mutateJSONResponse(t, http.StatusCreated, models.PermissionDTO{Username: "alice", VHost: "staging"}), nil
	}, &stdout)

	cmd := NewPermissionsCmd(rt)
	cmd.SetArgs([]string{"grant", "staging", "alice"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "alice")
	assert.Contains(t, stdout.String(), "staging")
}

func TestPermissionsRevokeCommand_SendsRequest(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, req.Method)
		require.Equal(t, "http://example.test/api/admin/permissions/staging/alice", req.URL.String())
		return mutateEmptyResponse(http.StatusNoContent), nil
	}, &stdout)

	cmd := NewPermissionsCmd(rt)
	cmd.SetArgs([]string{"revoke", "staging", "alice"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "alice")
	assert.Contains(t, stdout.String(), "staging")
}

func TestQueuesGetMessagesCommand_TextOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := newMutateRuntime(&RootOptions{
		BaseURL: "http://example.test",
		Token:   "jwt-token",
	}, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "http://example.test/api/queues/%2F/jobs/get", req.URL.String())

		var payload models.GetMessageRequest
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		assert.Equal(t, models.Reject, payload.AckMode)
		assert.Equal(t, 2, payload.MessageCount)

		return mutateJSONResponse(t, http.StatusOK, models.MessageListResponse{
			Messages: []models.MessageDTO{
				{DeliveryTag: 1, PayloadText: "hello", Redelivered: false},
				{DeliveryTag: 2, PayloadText: "world", Redelivered: true},
			},
		}), nil
	}, &stdout)

	cmd := NewQueuesCmd(rt)
	cmd.SetArgs([]string{"get-messages", "/", "jobs", "--count", "2", "--ack-mode", "reject"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "message[1]")
	assert.Contains(t, stdout.String(), "payload=hello")
	assert.Contains(t, stdout.String(), "redelivered=true")
}
