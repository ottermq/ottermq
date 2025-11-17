package vhost

import (
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/pkg/persistence"
)

func TestParseMaxLengthArgument_ValidInt64(t *testing.T) {
	args := map[string]interface{}{
		"x-max-length": int64(100),
	}

	maxLen, ok := parseMaxLengthArgument(args)
	if !ok {
		t.Error("Expected max-length to be parsed successfully")
	}
	if maxLen != 100 {
		t.Errorf("Expected max-length 100, got %d", maxLen)
	}
}

func TestParseMaxLengthArgument_ValidInt32(t *testing.T) {
	args := map[string]interface{}{
		"x-max-length": int32(50),
	}

	maxLen, ok := parseMaxLengthArgument(args)
	if !ok {
		t.Error("Expected max-length to be parsed successfully")
	}
	if maxLen != 50 {
		t.Errorf("Expected max-length 50, got %d", maxLen)
	}
}

func TestParseMaxLengthArgument_ValidInt(t *testing.T) {
	args := map[string]interface{}{
		"x-max-length": int(25),
	}

	maxLen, ok := parseMaxLengthArgument(args)
	if !ok {
		t.Error("Expected max-length to be parsed successfully")
	}
	if maxLen != 25 {
		t.Errorf("Expected max-length 25, got %d", maxLen)
	}
}

func TestParseMaxLengthArgument_ValidFloat64(t *testing.T) {
	args := map[string]interface{}{
		"x-max-length": float64(75),
	}

	maxLen, ok := parseMaxLengthArgument(args)
	if !ok {
		t.Error("Expected max-length to be parsed successfully")
	}
	if maxLen != 75 {
		t.Errorf("Expected max-length 75, got %d", maxLen)
	}
}

func TestParseMaxLengthArgument_Missing(t *testing.T) {
	args := map[string]interface{}{
		"other-arg": "value",
	}

	_, ok := parseMaxLengthArgument(args)
	if ok {
		t.Error("Expected max-length parsing to fail when x-max-length is missing")
	}
}

func TestParseMaxLengthArgument_InvalidType(t *testing.T) {
	args := map[string]interface{}{
		"x-max-length": "not-a-number",
	}

	_, ok := parseMaxLengthArgument(args)
	if ok {
		t.Error("Expected max-length parsing to fail for invalid type")
	}
}

func TestParseMaxLengthArgument_NegativeValue(t *testing.T) {
	args := map[string]interface{}{
		"x-max-length": int64(-10),
	}

	_, ok := parseMaxLengthArgument(args)
	if ok {
		t.Error("Expected max-length parsing to fail for negative value")
	}
}

func TestParseMaxLengthArgument_ZeroValue(t *testing.T) {
	args := map[string]interface{}{
		"x-max-length": int64(0),
	}

	_, ok := parseMaxLengthArgument(args)
	if ok {
		t.Error("Expected max-length parsing to fail for zero value")
	}
}

func TestNoOpQueueLengthLimiter_EnforceMaxLength(t *testing.T) {
	limiter := &NoOpQueueLengthLimiter{}
	queue := &Queue{
		Name:      "test-queue",
		maxLength: 10,
		messages:  make(chan Message, 100),
		count:     50, // Way over limit
	}

	// NoOp should not modify anything
	limiter.EnforceMaxLength(queue)

	// Count should be unchanged
	if queue.count != 50 {
		t.Errorf("NoOp should not change count, got %d", queue.count)
	}
}

func TestDefaultQueueLengthLimiter_EnforceMaxLength_NoLimit(t *testing.T) {
	vh := &VHost{
		Name: "test-vhost",
	}
	limiter := &DefaultQueueLengthLimiter{vh: vh}

	queue := &Queue{
		Name:      "test-queue",
		maxLength: 0, // No limit
		messages:  make(chan Message, 100),
		count:     0,
	}

	// Add some messages
	for i := 0; i < 20; i++ {
		queue.messages <- Message{
			ID:   string(rune(i)),
			Body: []byte("test"),
		}
		queue.count++
	}

	limiter.EnforceMaxLength(queue)

	// All messages should remain
	if queue.count != 20 {
		t.Errorf("Expected count 20, got %d", queue.count)
	}
}

func TestDefaultQueueLengthLimiter_EnforceMaxLength_WithinLimit(t *testing.T) {
	vh := &VHost{
		Name: "test-vhost",
	}
	limiter := &DefaultQueueLengthLimiter{vh: vh}

	queue := &Queue{
		Name:      "test-queue",
		maxLength: 10,
		messages:  make(chan Message, 100),
		count:     0,
	}

	// Add 5 messages (within limit)
	for i := 0; i < 5; i++ {
		queue.messages <- Message{
			ID:   string(rune(i)),
			Body: []byte("test"),
		}
		queue.count++
	}

	limiter.EnforceMaxLength(queue)

	// All messages should remain
	if queue.count != 5 {
		t.Errorf("Expected count 5, got %d", queue.count)
	}
}

func TestDefaultQueueLengthLimiter_EnforceMaxLength_ExactLimit(t *testing.T) {
	vh := &VHost{
		Name: "test-vhost",
	}
	limiter := &DefaultQueueLengthLimiter{vh: vh}

	queue := &Queue{
		Name:      "test-queue",
		maxLength: 10,
		messages:  make(chan Message, 100),
		count:     0,
	}

	// Add exactly 10 messages (at limit)
	for i := 0; i < 10; i++ {
		queue.messages <- Message{
			ID:   string(rune(i)),
			Body: []byte("test"),
		}
		queue.count++
	}

	limiter.EnforceMaxLength(queue)

	// Should evict 1 message to make room for incoming message
	// Leaving 9 messages in queue (ready to accept one more)
	if queue.count != 9 {
		t.Errorf("Expected count 9 after enforcement (to make room), got %d", queue.count)
	}
}

func TestDefaultQueueLengthLimiter_EnforceMaxLength_OverLimit(t *testing.T) {
	vh := &VHost{
		Name:    "test-vhost",
		Queues:  make(map[string]*Queue),
		persist: &MockPersistence{},
		ActiveExtensions: map[string]bool{
			"dlx": false, // No DLX for this test
		},
	}
	limiter := &DefaultQueueLengthLimiter{vh: vh}

	queue := &Queue{
		Name:      "test-queue",
		maxLength: 5,
		messages:  make(chan Message, 100),
		count:     0,
		Props:     &QueueProperties{Arguments: make(QueueArgs)},
	}

	// Add 10 messages (5 over limit)
	for i := 0; i < 10; i++ {
		queue.messages <- Message{
			ID:         string(rune('a' + i)),
			Body:       []byte("test"),
			EnqueuedAt: time.Now(),
			Properties: amqp.BasicProperties{},
		}
		queue.count++
	}

	limiter.EnforceMaxLength(queue)

	// Should evict 6 messages: (10 - 5) + 1 = 6 to evict
	// Leaving 4 messages in queue (ready to accept one more to reach limit of 5)
	if queue.count != 4 {
		t.Errorf("Expected count 4 after enforcement (to make room), got %d", queue.count)
	}

	// Verify the 6 oldest messages ('a' to 'f') were removed
	// Remaining should be 'g', 'h', 'i', 'j'
	remainingMsg := <-queue.messages
	queue.count--
	if remainingMsg.ID != "g" { // 'a' to 'f' removed, 'g' should be first
		t.Errorf("Expected oldest remaining message to have ID 'g', got '%s'", remainingMsg.ID)
	}
}

func TestDefaultQueueLengthLimiter_EnforceMaxLength_EmptyQueue(t *testing.T) {
	vh := &VHost{
		Name: "test-vhost",
	}
	limiter := &DefaultQueueLengthLimiter{vh: vh}

	queue := &Queue{
		Name:      "test-queue",
		maxLength: 5,
		messages:  make(chan Message, 100),
		count:     0, // Empty queue
	}

	// Should not panic or error
	limiter.EnforceMaxLength(queue)

	if queue.count != 0 {
		t.Errorf("Expected count 0, got %d", queue.count)
	}
}

func TestDefaultQueueLengthLimiter_EnforceMaxLength_LargeExcess(t *testing.T) {
	vh := &VHost{
		Name:    "test-vhost",
		Queues:  make(map[string]*Queue),
		persist: &MockPersistence{},
		ActiveExtensions: map[string]bool{
			"dlx": false,
		},
	}
	limiter := &DefaultQueueLengthLimiter{vh: vh}

	queue := &Queue{
		Name:      "test-queue",
		maxLength: 10,
		messages:  make(chan Message, 1000),
		count:     0,
		Props:     &QueueProperties{Arguments: make(QueueArgs)},
	}

	// Add 100 messages (90 over limit)
	for i := 0; i < 100; i++ {
		queue.messages <- Message{
			ID:         string(rune(i)),
			Body:       []byte("test"),
			EnqueuedAt: time.Now(),
			Properties: amqp.BasicProperties{},
		}
		queue.count++
	}

	limiter.EnforceMaxLength(queue)

	// Should evict 91 messages: (100 - 10) + 1 = 91
	// Leaving 9 messages in queue (ready to accept one more)
	if queue.count != 9 {
		t.Errorf("Expected count 9 after enforcement (to make room), got %d", queue.count)
	}
}

// MockPersistence is a simple mock for testing
type MockPersistence struct{}

func (m *MockPersistence) SaveQueueMetadata(vhost, name string, props persistence.QueueProperties) error {
	return nil
}

func (m *MockPersistence) LoadQueueMetadata(vhost, name string) (persistence.QueueProperties, error) {
	return persistence.QueueProperties{}, nil
}

func (m *MockPersistence) DeleteQueueMetadata(vhost, name string) error {
	return nil
}

func (m *MockPersistence) SaveExchangeMetadata(vhost, name, exchangeType string, props persistence.ExchangeProperties) error {
	return nil
}

func (m *MockPersistence) LoadExchangeMetadata(vhost, name string) (string, persistence.ExchangeProperties, error) {
	return "", persistence.ExchangeProperties{}, nil
}

func (m *MockPersistence) DeleteExchangeMetadata(vhost, name string) error {
	return nil
}

func (m *MockPersistence) SaveBindingState(vhost, exchange, queue, routingKey string, arguments map[string]interface{}) error {
	return nil
}

func (m *MockPersistence) LoadExchangeBindings(vhost, exchange string) ([]persistence.BindingData, error) {
	return nil, nil
}

func (m *MockPersistence) DeleteBindingState(vhost, exchange, queue, routingKey string, arguments map[string]interface{}) error {
	return nil
}

func (m *MockPersistence) SaveMessage(vhost, queue, msgId string, msgBody []byte, msgProps persistence.MessageProperties) error {
	return nil
}

func (m *MockPersistence) LoadMessages(vhostName, queueName string) ([]persistence.Message, error) {
	return nil, nil
}

func (m *MockPersistence) DeleteMessage(vhost, queue, msgID string) error {
	return nil
}

func (m *MockPersistence) LoadAllExchanges(vhost string) ([]persistence.ExchangeSnapshot, error) {
	return nil, nil
}

func (m *MockPersistence) LoadAllQueues(vhost string) ([]persistence.QueueSnapshot, error) {
	return nil, nil
}

func (m *MockPersistence) Initialize() error {
	return nil
}

func (m *MockPersistence) Close() error {
	return nil
}
