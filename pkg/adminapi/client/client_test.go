package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(t *testing.T, fn roundTripFunc) *Client {
	t.Helper()

	httpClient := &http.Client{
		Transport: fn,
	}

	return New("http://example.test", httpClient)
}

func jsonResponse(t *testing.T, status int, body any) *http.Response {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

func emptyResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}
}

func TestNew_UsesDefaults(t *testing.T) {
	c := New("", nil)

	assert.Equal(t, DefaultBaseURL, c.BaseURL())
	assert.Equal(t, DefaultAPIPrefix, c.APIPrefix())
	assert.Empty(t, c.Token())
}

func TestLogin_SendsCredentialsAndStoresToken(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "http://example.test/api/login", r.URL.String())

		var req models.AuthRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "guest", req.Username)
		assert.Equal(t, "guest", req.Password)

		return jsonResponse(t, http.StatusOK, models.AuthResponse{Token: "jwt-token"}), nil
	})

	resp, err := c.Login(context.Background(), "guest", "guest")
	require.NoError(t, err)

	require.NotNil(t, resp)
	assert.Equal(t, "jwt-token", resp.Token)
	assert.Equal(t, "jwt-token", c.Token())
}

func TestGetOverview_UsesBearerToken(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "http://example.test/api/overview", r.URL.String())
		assert.Equal(t, "Bearer jwt-token", r.Header.Get("Authorization"))

		return jsonResponse(t, http.StatusOK, models.OverviewDTO{
			BrokerDetails: models.OverviewBrokerDetails{
				Product: "OtterMQ",
				Version: "test",
			},
		}), nil
	})
	c.SetToken("jwt-token")

	resp, err := c.GetOverview(context.Background())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "OtterMQ", resp.BrokerDetails.Product)
}

func TestGetQueue_PathEscapesVHostAndQueue(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "http://example.test/api/queues/%2F/my.queue", r.URL.String())

		return jsonResponse(t, http.StatusOK, models.QueueDTO{
			VHost: "/",
			Name:  "my.queue",
		}), nil
	})

	resp, err := c.GetQueue(context.Background(), "/", "my.queue")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "my.queue", resp.Name)
}

func TestListBindings_UsesOptionalVHostPath(t *testing.T) {
	tests := []struct {
		name        string
		vhost       string
		expectedURL string
	}{
		{name: "global", vhost: "", expectedURL: "http://example.test/api/bindings"},
		{name: "vhost scoped", vhost: "/", expectedURL: "http://example.test/api/bindings/%2F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
				require.Equal(t, tt.expectedURL, r.URL.String())

				return jsonResponse(t, http.StatusOK, models.BindingListResponse{
					Bindings: []models.BindingDTO{{Source: "amq.direct"}},
				}), nil
			})

			resp, err := c.ListBindings(context.Background(), tt.vhost)
			require.NoError(t, err)
			require.Len(t, resp, 1)
			assert.Equal(t, "amq.direct", resp[0].Source)
		})
	}
}

func TestCreateQueue_UsesGeneratedNameRouteWhenQueueEmpty(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "http://example.test/api/queues/%2F/", r.URL.String())

		var req models.CreateQueueRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.True(t, req.Durable)

		return jsonResponse(t, http.StatusOK, models.SuccessResponse{Message: "Queue 'amq.gen-1' created successfully"}), nil
	})

	resp, err := c.CreateQueue(context.Background(), "/", "", models.CreateQueueRequest{Durable: true})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.Message, "created successfully")
}

func TestDeleteQueue_UsesConditionalQueryParams(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "http://example.test/api/queues/%2F/jobs?ifEmpty=true&ifUnused=true", r.URL.String())
		return emptyResponse(http.StatusNoContent), nil
	})

	err := c.DeleteQueue(context.Background(), "/", "jobs", true, true)
	require.NoError(t, err)
}

func TestCreateExchange_SendsExpectedPayload(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "http://example.test/api/exchanges/%2F/events", r.URL.String())

		var req models.CreateExchangeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "topic", req.ExchangeType)
		assert.True(t, req.Durable)

		return jsonResponse(t, http.StatusOK, models.SuccessResponse{Message: "Exchange created successfully"}), nil
	})

	resp, err := c.CreateExchange(context.Background(), "/", "events", models.CreateExchangeRequest{
		ExchangeType: "topic",
		Durable:      true,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Exchange created successfully", resp.Message)
}

func TestCreateBinding_SendsRequestBody(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "http://example.test/api/bindings", r.URL.String())

		var req models.CreateBindingRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "/", req.VHost)
		assert.Equal(t, "amq.direct", req.Source)
		assert.Equal(t, "jobs", req.Destination)

		return jsonResponse(t, http.StatusOK, models.SuccessResponse{Message: "Queue bound to exchange"}), nil
	})

	resp, err := c.CreateBinding(context.Background(), models.CreateBindingRequest{
		VHost:       "/",
		Source:      "amq.direct",
		Destination: "jobs",
		RoutingKey:  "jobs",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Queue bound to exchange", resp.Message)
}

func TestPublishMessage_UsesExchangePublishRoute(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "http://example.test/api/exchanges/%2F/events/publish", r.URL.String())

		var req models.PublishMessageRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "jobs.created", req.RoutingKey)
		assert.Equal(t, "hello", req.Payload)

		return jsonResponse(t, http.StatusOK, models.SuccessResponse{Message: "Message published"}), nil
	})

	resp, err := c.PublishMessage(context.Background(), "/", "events", models.PublishMessageRequest{
		RoutingKey: "jobs.created",
		Payload:    "hello",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Message published", resp.Message)
}

func TestPublishMessage_UsesDefaultExchangeAliasWhenExchangeEmpty(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "http://example.test/api/exchanges/%2F/%28AMQP%20default%29/publish", r.URL.String())

		var req models.PublishMessageRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "bonjour", req.RoutingKey)
		assert.Equal(t, "hello", req.Payload)

		return jsonResponse(t, http.StatusOK, models.SuccessResponse{Message: "Message published"}), nil
	})

	resp, err := c.PublishMessage(context.Background(), "/", "", models.PublishMessageRequest{
		RoutingKey: "bonjour",
		Payload:    "hello",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Message published", resp.Message)
}

func TestGetMessages_UsesQueueGetRoute(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "http://example.test/api/queues/%2F/jobs/get", r.URL.String())

		var req models.GetMessageRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, models.RejectRequeue, req.AckMode)
		assert.Equal(t, 2, req.MessageCount)

		return jsonResponse(t, http.StatusOK, models.MessageListResponse{
			Messages: []models.MessageDTO{{PayloadText: "hello"}},
		}), nil
	})

	resp, err := c.GetMessages(context.Background(), "/", "jobs", models.GetMessageRequest{
		AckMode:      models.RejectRequeue,
		MessageCount: 2,
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, "hello", resp[0].PayloadText)
}

func TestAPIError_DecodesErrorResponse(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusUnauthorized, models.ErrorResponse{
			Error: "invalid credentials",
		}), nil
	})

	_, err := c.Login(context.Background(), "guest", "wrong")
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, "invalid credentials", apiErr.Message)
}
