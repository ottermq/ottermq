package memento

import "github.com/andrelcunha/ottermq/pkg/persistence"

// MementoPersistence implements persistence.Persistence with no-ops for placeholders.
type MementoPersistence struct {
}

func (m *MementoPersistence) SaveExchangeMetadata(vhost, name, exchangeType string, props persistence.ExchangeProperties) error {
	return nil
}
func (m *MementoPersistence) LoadExchangeMetadata(vhost, name string) (string, persistence.ExchangeProperties, error) {
	return "", persistence.ExchangeProperties{}, nil
}
func (m *MementoPersistence) DeleteExchangeMetadata(vhost, name string) error { return nil }
func (m *MementoPersistence) SaveQueueMetadata(vhost, name string, props persistence.QueueProperties) error {
	return nil
}
func (m *MementoPersistence) LoadQueueMetadata(vhost, name string) (persistence.QueueProperties, error) {
	return persistence.QueueProperties{}, nil
}
func (m *MementoPersistence) DeleteQueueMetadata(vhost, name string) error { return nil }
func (m *MementoPersistence) SaveBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	return nil
}
func (m *MementoPersistence) LoadExchangeBindings(vhost, exchange string) ([]persistence.BindingData, error) {
	return nil, nil
}
func (m *MementoPersistence) DeleteBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	return nil
}
func (m *MementoPersistence) LoadAllExchanges(vhost string) ([]persistence.ExchangeSnapshot, error) {
	return nil, nil
}
func (m *MementoPersistence) LoadAllQueues(vhost string) ([]persistence.QueueSnapshot, error) {
	return nil, nil
}
func (m *MementoPersistence) Initialize() error { return nil }
func (m *MementoPersistence) Close() error      { return nil }

// LoadMessages returns an empty slice
func (m *MementoPersistence) LoadMessages(vhost, queue string) ([]persistence.Message, error) {
	return []persistence.Message{}, nil
}

// SaveMessage is a no-op
func (m *MementoPersistence) SaveMessage(vhost, queue, msgId string, msgBody []byte, msgProps persistence.MessageProperties) error {
	return nil
}

// DeleteMessage optionally tracks deleted messages if DeletedMessages or DeletedMessagesDetailed slices are initialized
func (m *MementoPersistence) DeleteMessage(vhost, queue, msgId string) error {
	return nil
}
