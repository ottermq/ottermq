package dummy

import "github.com/andrelcunha/ottermq/pkg/persistence"

// DummyPersistence implements persistence.Persistence with no-ops for testing
type DummyPersistence struct{}

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

// SaveMessage
func (d *DummyPersistence) SaveMessage(vhost, queue, msgId string, msgBody []byte, msgProps persistence.MessageProperties) error {
	return nil
}
func (d *DummyPersistence) DeleteMessage(vhost, queue, msgId string) error {
	return nil
}
