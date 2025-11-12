package dummy

import "github.com/andrelcunha/ottermq/pkg/persistence"

// DummyPersistence implements persistence.Persistence with no-ops for testing.
// It can optionally track DeleteMessage calls for test assertions.
type DummyPersistence struct {
	// DeletedMessages tracks msgId of deleted messages (if tracking enabled)
	DeletedMessages []string
	// DeletedMessagesDetailed tracks full details of deleted messages (if tracking enabled)
	DeletedMessagesDetailed []DeleteRecord
}

// DeleteRecord captures full details of a DeleteMessage call
type DeleteRecord struct {
	Vhost string
	Queue string
	MsgID string
}

func (d *DummyPersistence) SaveExchangeMetadata(vhost, name, exchangeType string, props persistence.ExchangeProperties) error {
	return nil
}
func (d *DummyPersistence) LoadExchangeMetadata(vhost, name string) (string, persistence.ExchangeProperties, error) {
	return "", persistence.ExchangeProperties{}, nil
}
func (d *DummyPersistence) DeleteExchangeMetadata(vhost, name string) error { return nil }
func (d *DummyPersistence) SaveQueueMetadata(vhost, name string, props persistence.QueueProperties) error {
	return nil
}
func (d *DummyPersistence) LoadQueueMetadata(vhost, name string) (persistence.QueueProperties, error) {
	return persistence.QueueProperties{}, nil
}
func (d *DummyPersistence) DeleteQueueMetadata(vhost, name string) error { return nil }
func (d *DummyPersistence) SaveBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	return nil
}
func (d *DummyPersistence) LoadExchangeBindings(vhost, exchange string) ([]persistence.BindingData, error) {
	return nil, nil
}
func (d *DummyPersistence) DeleteBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	return nil
}
func (d *DummyPersistence) LoadAllExchanges(vhost string) ([]persistence.ExchangeSnapshot, error) {
	return nil, nil
}
func (d *DummyPersistence) LoadAllQueues(vhost string) ([]persistence.QueueSnapshot, error) {
	return nil, nil
}
func (d *DummyPersistence) Initialize() error { return nil }
func (d *DummyPersistence) Close() error      { return nil }

// LoadMessages returns an empty slice
func (d *DummyPersistence) LoadMessages(vhost, queue string) ([]persistence.Message, error) {
	return []persistence.Message{}, nil
}

// SaveMessage is a no-op
func (d *DummyPersistence) SaveMessage(vhost, queue, msgId string, msgBody []byte, msgProps persistence.MessageProperties) error {
	return nil
}

// DeleteMessage optionally tracks deleted messages if DeletedMessages or DeletedMessagesDetailed slices are initialized
func (d *DummyPersistence) DeleteMessage(vhost, queue, msgId string) error {
	if d.DeletedMessages != nil {
		d.DeletedMessages = append(d.DeletedMessages, msgId)
	}
	if d.DeletedMessagesDetailed != nil {
		d.DeletedMessagesDetailed = append(d.DeletedMessagesDetailed, DeleteRecord{
			Vhost: vhost,
			Queue: queue,
			MsgID: msgId,
		})
	}
	return nil
}
