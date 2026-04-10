package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ottermq/ottermq/internal/core/amqp"
	"github.com/ottermq/ottermq/internal/core/broker"
	"github.com/ottermq/ottermq/internal/core/broker/management"
	"github.com/ottermq/ottermq/internal/core/broker/vhost"
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/ottermq/ottermq/pkg/metrics"
	"github.com/ottermq/ottermq/pkg/persistence/implementations/dummy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHTTPBroker() *broker.Broker {
	options := vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
		EnableDLX:       false,
		EnableTTL:       false,
		EnableQLL:       false,
	}

	vh := vhost.NewVhost("/", options)
	vh.SetFramer(&amqp.DefaultFramer{})
	vh.SetMetricsCollector(metrics.NewMockCollector(nil))

	b := &broker.Broker{
		VHosts:     map[string]*vhost.VHost{"/": vh},
		Management: nil,
	}
	b.Management = management.NewService(b)
	return b
}

func TestPublishMessage_DecodesEncodedVHostAndDefaultExchangeAlias(t *testing.T) {
	b := newTestHTTPBroker()
	vh := b.GetVHost("/")
	require.NotNil(t, vh)

	_, err := vh.CreateQueue("bonjour", &vhost.QueueProperties{}, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)
	err = vh.BindQueue("", "bonjour", "bonjour", nil, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	app := fiber.New()
	app.Post("/api/exchanges/:vhost/:exchange/publish", func(c *fiber.Ctx) error {
		return PublishMessage(c, b)
	})

	body, err := json.Marshal(models.PublishMessageRequest{
		RoutingKey: "bonjour",
		Payload:    "hello",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/exchanges/%2F/%28AMQP%20default%29/publish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	count, err := vh.GetMessageCount("bonjour")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
