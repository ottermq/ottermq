package management

import (
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListConsumers_MultipleQueues(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	vh := broker.GetVHost("/")

	// Add consumers to different queues
	vh.ConsumersByQueue["queue1"] = []*vhost.Consumer{
		{
			Tag:           "consumer1",
			Channel:       1,
			QueueName:     "queue1",
			Connection:    nil,
			Active:        true,
			PrefetchCount: 0,
			Props: &vhost.ConsumerProperties{
				NoAck:     false,
				Exclusive: false,
				Arguments: map[string]interface{}{},
			},
		},
	}
	vh.ConsumersByQueue["queue2"] = []*vhost.Consumer{
		{
			Tag:           "consumer2",
			Channel:       1,
			QueueName:     "queue2",
			Connection:    nil,
			Active:        true,
			PrefetchCount: 0,
			Props: &vhost.ConsumerProperties{
				NoAck:     false,
				Exclusive: false,
				Arguments: map[string]interface{}{},
			},
		},
		{
			Tag:           "consumer3",
			Channel:       2,
			QueueName:     "queue2",
			Connection:    nil,
			Active:        true,
			PrefetchCount: 0,
			Props: &vhost.ConsumerProperties{
				NoAck:     false,
				Exclusive: false,
				Arguments: map[string]interface{}{},
			},
		},
	}

	consumers, err := service.ListConsumers("/")
	require.NoError(t, err)
	assert.Len(t, consumers, 3)
}

func TestListQueueConsumers_EmptyQueue(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	consumers, err := service.ListQueueConsumers("/", "empty-queue")
	require.NoError(t, err)
	assert.Empty(t, consumers)
}
