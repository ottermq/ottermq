package management

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateExchange_AllTypes(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	types := []string{"direct", "fanout", "topic"}
	// "headers" might not be fully supported in validation or parsing yet, checking CreateExchangeRequest validation
	// The request struct has `validate:"required,oneof=direct fanout topic headers"`
	// But ParseExchangeType in exchange.go only switches on direct, fanout, topic.
	// Let's stick to supported types.

	for _, typ := range types {
		exName := "test-" + typ
		t.Run(typ, func(t *testing.T) {
			req := models.CreateExchangeRequest{
				ExchangeType: typ,
				Durable:      true,
				AutoDelete:   false,
				Arguments:    nil,
			}
			dto, err := service.CreateExchange("/", exName, req)
			require.NoError(t, err)
			assert.Equal(t, exName, dto.Name)
			assert.Equal(t, typ, dto.Type)
			assert.True(t, dto.Durable)
		})
	}
}

func TestDeleteExchange_IfUnused(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create exchange
	exName := "test-exchange"
	_, err := service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
	})
	require.NoError(t, err)

	// Create queue
	qName := "test-queue"
	_, err = service.CreateQueue("/", qName, models.CreateQueueRequest{})
	require.NoError(t, err)

	// Bind queue to exchange to make it "used"
	// Since management service bindings are Phase 3, we use the vhost directly
	vh := broker.GetVHost("/")
	// We need to use vhost package to access BindQueue if we were using types,
	// but here we just call the method on the interface/struct.
	// The error "imported and not used" suggests we didn't use `vhost.` anywhere.
	// We can remove the import if we don't use types from it,
	// OR we can use a type from it to silence the error.
	// Let's use vhost.VHost type assertion or similar if needed,
	// but actually we just need to call BindQueue.
	// Wait, BindQueue is a method of *vhost.VHost.
	// broker.GetVHost returns *vhost.VHost.
	// So we are using the package implicitly but not explicitly referring to `vhost` symbol.
	// Let's remove the import if it's not used.
	err = vh.BindQueue(exName, qName, "key", nil, vhost.MANAGEMENT_CONNECTION_ID)
	require.NoError(t, err)

	// Try delete with ifUnused=true (should fail)
	err = service.DeleteExchange("/", exName, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has active bindings")

	// Delete queue (should remove bindings)
	err = service.DeleteQueue("/", qName, false, false)
	require.NoError(t, err)

	// Try delete again (should succeed)
	err = service.DeleteExchange("/", exName, true)
	assert.NoError(t, err)
}

func TestGetExchange(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	exName := "my-exchange"
	req := models.CreateExchangeRequest{
		ExchangeType: "topic",
		Durable:      true,
		Arguments:    map[string]any{"x-custom": "value"},
	}
	_, err := service.CreateExchange("/", exName, req)
	require.NoError(t, err)

	dto, err := service.GetExchange("/", exName)
	require.NoError(t, err)
	assert.Equal(t, exName, dto.Name)
	assert.Equal(t, "topic", dto.Type)
	assert.Equal(t, "value", dto.Arguments["x-custom"])
}

func TestListExchanges(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Should have default exchanges initially
	initialList, err := service.ListExchanges()
	require.NoError(t, err)
	initialCount := len(initialList)

	// Create a new one
	exName := "list-test-exchange"
	_, err = service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
	})
	require.NoError(t, err)

	// List again
	newList, err := service.ListExchanges()
	require.NoError(t, err)
	assert.Equal(t, initialCount+1, len(newList))

	found := false
	for _, ex := range newList {
		if ex.Name == exName {
			found = true
			break
		}
	}
	assert.True(t, found, "Created exchange not found in list")
}

func TestCreateExchange_Idempotency(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	// Create exchange
	exName := "idempotent-exchange"
	_, err := service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
		Durable:      true,
	})
	require.NoError(t, err)

	// Create again with same props
	_, err = service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "direct",
		Durable:      true,
	})
	require.NoError(t, err)

	// Create again with different props (should fail)
	_, err = service.CreateExchange("/", exName, models.CreateExchangeRequest{
		ExchangeType: "fanout", // Different type
		Durable:      true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "different type")
}
