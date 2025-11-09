package vhost

import (
	"net"
	"strings"
	"testing"
	"time"
)

// MockConnection implements net.Conn for testing
type MockConnection struct {
	localAddr  net.Addr
	remoteAddr net.Addr
	closed     bool
	id         string
}

func NewMockConnection(id string) *MockConnection {
	return &MockConnection{
		localAddr:  &MockAddr{"tcp", "127.0.0.1:5672"},
		remoteAddr: &MockAddr{"tcp", "127.0.0.1:12345"},
		id:         id,
	}
}

func (m *MockConnection) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *MockConnection) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *MockConnection) Close() error                       { m.closed = true; return nil }
func (m *MockConnection) LocalAddr() net.Addr                { return m.localAddr }
func (m *MockConnection) RemoteAddr() net.Addr               { return m.remoteAddr }
func (m *MockConnection) SetDeadline(t time.Time) error      { return nil }
func (m *MockConnection) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockConnection) SetWriteDeadline(t time.Time) error { return nil }

// MockAddr implements net.Addr for testing
type MockAddr struct {
	network string
	address string
}

func (m *MockAddr) Network() string { return m.network }
func (m *MockAddr) String() string  { return m.address }

func createTestVHost() *VHost {
	vh := NewVhost("test-vhost", 1000, nil) // Using nil persistence for tests

	// Add test queue
	vh.Queues["test-queue"] = &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Passive:    false,
			Durable:    false,
			AutoDelete: false,
			Exclusive:  false,
			Arguments:  nil,
		},
	}

	// Add exclusive test queue
	vh.Queues["exclusive-queue"] = &Queue{
		Name: "exclusive-queue",
		Props: &QueueProperties{
			Passive:    false,
			Durable:    false,
			AutoDelete: false,
			Exclusive:  true,
			Arguments:  nil,
		},
	}

	return vh
}

func TestRegisterConsumer_ValidConsumer(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	consumer := NewConsumer(conn, 1, "test-queue", "test-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(consumer)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check primary registry
	key := ConsumerKey{Channel: 1, Tag: "test-consumer"}
	registeredConsumer, exists := vh.Consumers[key]
	if !exists {
		t.Error("Consumer not found in primary registry")
	} else {
		if !registeredConsumer.Active {
			t.Error("Consumer should be active")
		}
		if registeredConsumer.QueueName != "test-queue" {
			t.Errorf("Expected queue name 'test-queue', got '%s'", registeredConsumer.QueueName)
		}
	}

	// Check queue index
	queueConsumers := vh.ConsumersByQueue["test-queue"]
	if len(queueConsumers) != 1 {
		t.Errorf("Expected 1 consumer in queue index, got %d", len(queueConsumers))
	} else if queueConsumers[0].Tag != "test-consumer" {
		t.Errorf("Expected consumer tag 'test-consumer', got '%s'", queueConsumers[0].Tag)
	}

	// Check channel index
	channelKey := ConnectionChannelKey{conn, 1}
	channelConsumers := vh.ConsumersByChannel[channelKey]
	if len(channelConsumers) != 1 {
		t.Errorf("Expected 1 consumer in channel index, got %d", len(channelConsumers))
	} else if channelConsumers[0].Tag != "test-consumer" {
		t.Errorf("Expected consumer tag 'test-consumer', got '%s'", channelConsumers[0].Tag)
	}
}

func TestRegisterConsumer_NonExistentQueue(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	consumer := NewConsumer(conn, 1, "non-existent-queue", "test-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(consumer)

	if err == nil {
		t.Error("Expected error for non-existent queue")
	}

	expectedSubstring := "non-existent-queue"
	if err != nil && !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedSubstring, err.Error())
	}

	// Check that consumer was not registered
	if len(vh.Consumers) != 0 {
		t.Errorf("Expected 0 consumers, got %d", len(vh.Consumers))
	}
}

func TestRegisterConsumer_DuplicateConsumer(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register first consumer
	consumer1 := NewConsumer(conn, 1, "test-queue", "duplicate-tag", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register first consumer: %v", err)
	}

	// Try to register second consumer with same tag and channel
	consumer2 := NewConsumer(conn, 1, "test-queue", "duplicate-tag", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err = vh.RegisterConsumer(consumer2)

	if err == nil {
		t.Error("Expected error for duplicate consumer")
	}

	expectedError := "consumer with tag duplicate-tag already exists on channel 1"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Check that only one consumer exists
	if len(vh.Consumers) != 1 {
		t.Errorf("Expected 1 consumer, got %d", len(vh.Consumers))
	}
}

func TestRegisterConsumer_ExclusiveConsumerExists(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register exclusive consumer
	exclusiveConsumer := NewConsumer(conn, 1, "test-queue", "exclusive-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: true,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(exclusiveConsumer)
	if err != nil {
		t.Fatalf("Failed to register exclusive consumer: %v", err)
	}

	// Try to register another consumer on the same queue
	normalConsumer := NewConsumer(conn, 2, "test-queue", "normal-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err = vh.RegisterConsumer(normalConsumer)

	if err == nil {
		t.Error("Expected error when trying to add consumer to queue with exclusive consumer")
	}

	expectedError := "exclusive consumer already exists for queue test-queue"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRegisterConsumer_ExclusiveConsumerWithExistingConsumers(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register normal consumer first
	normalConsumer := NewConsumer(conn, 1, "test-queue", "normal-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(normalConsumer)
	if err != nil {
		t.Fatalf("Failed to register normal consumer: %v", err)
	}

	// Try to register exclusive consumer when others exist
	exclusiveConsumer := NewConsumer(conn, 2, "test-queue", "exclusive-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: true,
		Arguments: nil,
	})

	err = vh.RegisterConsumer(exclusiveConsumer)

	if err == nil {
		t.Error("Expected error when trying to add exclusive consumer when other consumers exist")
	}

	expectedError := "cannot add exclusive consumer when other consumers exist for queue test-queue"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRegisterConsumer_MultipleConsumersOnDifferentChannels(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register consumer on channel 1
	consumer1 := NewConsumer(conn, 1, "test-queue", "consumer-1", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register consumer 1: %v", err)
	}

	// Register consumer on channel 2
	consumer2 := NewConsumer(conn, 2, "test-queue", "consumer-2", &ConsumerProperties{
		NoAck:     true,
		Exclusive: false,
		Arguments: nil,
	})

	err = vh.RegisterConsumer(consumer2)
	if err != nil {
		t.Fatalf("Failed to register consumer 2: %v", err)
	}

	// Check that both consumers are registered
	if len(vh.Consumers) != 2 {
		t.Errorf("Expected 2 consumers, got %d", len(vh.Consumers))
	}

	// Check queue index has both consumers
	queueConsumers := vh.ConsumersByQueue["test-queue"]
	if len(queueConsumers) != 2 {
		t.Errorf("Expected 2 consumers in queue index, got %d", len(queueConsumers))
	}

	// Check channel indexes
	channel1Key := ConnectionChannelKey{conn, 1}
	channel1Consumers := vh.ConsumersByChannel[channel1Key]
	if len(channel1Consumers) != 1 {
		t.Errorf("Expected 1 consumer on channel 1, got %d", len(channel1Consumers))
	}

	channel2Key := ConnectionChannelKey{conn, 2}
	channel2Consumers := vh.ConsumersByChannel[channel2Key]
	if len(channel2Consumers) != 1 {
		t.Errorf("Expected 1 consumer on channel 2, got %d", len(channel2Consumers))
	}
}

func TestCancelConsumer_ValidConsumer(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register consumer first
	consumer := NewConsumer(conn, 1, "test-queue", "test-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(consumer)
	if err != nil {
		t.Fatalf("Failed to register consumer: %v", err)
	}

	// Cancel the consumer
	err = vh.CancelConsumer(1, "test-consumer")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that consumer was removed from all indexes
	key := ConsumerKey{Channel: 1, Tag: "test-consumer"}
	if _, exists := vh.Consumers[key]; exists {
		t.Error("Consumer should have been removed from primary registry")
	}

	queueConsumers := vh.ConsumersByQueue["test-queue"]
	if len(queueConsumers) != 0 {
		t.Errorf("Expected 0 consumers in queue index, got %d", len(queueConsumers))
	}

	channelKey := ConnectionChannelKey{conn, 1}
	channelConsumers := vh.ConsumersByChannel[channelKey]
	if len(channelConsumers) != 0 {
		t.Errorf("Expected 0 consumers in channel index, got %d", len(channelConsumers))
	}
}

func TestCancelConsumer_NonExistentConsumer(t *testing.T) {
	vh := createTestVHost()

	err := vh.CancelConsumer(1, "non-existent-consumer")

	if err == nil {
		t.Error("Expected error for non-existent consumer")
	}

	expectedError := "consumer with tag non-existent-consumer on channel 1 does not exist"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestCleanupChannel(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register multiple consumers on the same channel
	consumer1 := NewConsumer(conn, 1, "test-queue", "consumer-1", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	consumer2 := NewConsumer(conn, 1, "test-queue", "consumer-2", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register consumer 1: %v", err)
	}

	err = vh.RegisterConsumer(consumer2)
	if err != nil {
		t.Fatalf("Failed to register consumer 2: %v", err)
	}

	// Verify consumers are registered
	if len(vh.Consumers) != 2 {
		t.Fatalf("Expected 2 consumers before cleanup, got %d", len(vh.Consumers))
	}

	// Cleanup the channel
	vh.CleanupChannel(conn, 1)

	// Check that all consumers on the channel were removed
	if len(vh.Consumers) != 0 {
		t.Errorf("Expected 0 consumers after cleanup, got %d", len(vh.Consumers))
	}

	// Check that queue index is cleaned up
	queueConsumers := vh.ConsumersByQueue["test-queue"]
	if len(queueConsumers) != 0 {
		t.Errorf("Expected 0 consumers in queue index after cleanup, got %d", len(queueConsumers))
	}

	// Check that channel index is cleaned up
	channelKey := ConnectionChannelKey{conn, 1}
	if _, exists := vh.ConsumersByChannel[channelKey]; exists {
		t.Error("Channel index should have been cleaned up")
	}
}

func TestCleanupConnection(t *testing.T) {
	vh := createTestVHost()
	conn1 := NewMockConnection("conn-1")
	conn2 := NewMockConnection("conn-2")

	// Register consumers on different connections and channels
	consumer1 := NewConsumer(conn1, 1, "test-queue", "consumer-1", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	consumer2 := NewConsumer(conn1, 2, "test-queue", "consumer-2", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	consumer3 := NewConsumer(conn2, 1, "test-queue", "consumer-3", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register consumer 1: %v", err)
	}

	err = vh.RegisterConsumer(consumer2)
	if err != nil {
		t.Fatalf("Failed to register consumer 2: %v", err)
	}

	err = vh.RegisterConsumer(consumer3)
	if err != nil {
		t.Fatalf("Failed to register consumer 3: %v", err)
	}

	// Verify all consumers are registered
	if len(vh.Consumers) != 3 {
		t.Fatalf("Expected 3 consumers before cleanup, got %d", len(vh.Consumers))
	}

	// Cleanup connection 1
	vh.CleanupConnection(conn1)

	// Check that only consumers from conn1 were removed
	if len(vh.Consumers) != 1 {
		t.Errorf("Expected 1 consumer after cleanup (from conn2), got %d", len(vh.Consumers))
	}

	// Check that remaining consumer is from conn2
	for _, consumer := range vh.Consumers {
		if consumer.Connection != conn2 {
			t.Error("Remaining consumer should be from conn2")
		}
	}

	// Check that queue index still has one consumer
	queueConsumers := vh.ConsumersByQueue["test-queue"]
	if len(queueConsumers) != 1 {
		t.Errorf("Expected 1 consumer in queue index after cleanup, got %d", len(queueConsumers))
	}
}

func TestConnectionScopedChannels_BugFix(t *testing.T) {
	// This test demonstrates the fix for the bug where CleanupChannel
	// would clean up channels with the same number from different connections
	vh := createTestVHost()
	connA := NewMockConnection("conn-A")
	connB := NewMockConnection("conn-B")

	// Both connections have consumers on channel 1 (same channel number!)
	consumerA1 := NewConsumer(connA, 1, "test-queue", "consumer-A1", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	consumerB1 := NewConsumer(connB, 1, "test-queue", "consumer-B1", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	// Register both consumers
	err := vh.RegisterConsumer(consumerA1)
	if err != nil {
		t.Fatalf("Failed to register consumer A1: %v", err)
	}

	err = vh.RegisterConsumer(consumerB1)
	if err != nil {
		t.Fatalf("Failed to register consumer B1: %v", err)
	}

	// Verify both consumers are registered
	if len(vh.Consumers) != 2 {
		t.Fatalf("Expected 2 consumers, got %d", len(vh.Consumers))
	}

	// Cleanup channel 1 on connection A only
	vh.CleanupChannel(connA, 1)

	// BUG FIX VERIFICATION: Only consumer A1 should be removed, B1 should remain
	if len(vh.Consumers) != 1 {
		t.Errorf("Expected 1 consumer remaining after cleanup, got %d", len(vh.Consumers))
	}

	// Check that consumer B1 still exists
	keyB := ConsumerKey{Channel: 1, Tag: "consumer-B1"}
	remainingConsumer, exists := vh.Consumers[keyB]
	if !exists {
		t.Error("Consumer B1 should still exist after cleaning up channel 1 on connection A")
	} else if remainingConsumer.Connection != connB {
		t.Error("Remaining consumer should be from connection B")
	}

	// Check that consumer A1 was actually removed
	keyA := ConsumerKey{Channel: 1, Tag: "consumer-A1"}
	if _, exists := vh.Consumers[keyA]; exists {
		t.Error("Consumer A1 should have been removed")
	}

	// Check connection-scoped channel indexes
	channelKeyA := ConnectionChannelKey{connA, 1}
	channelKeyB := ConnectionChannelKey{connB, 1}

	if _, exists := vh.ConsumersByChannel[channelKeyA]; exists {
		t.Error("Channel index for connection A should have been cleaned up")
	}

	if len(vh.ConsumersByChannel[channelKeyB]) != 1 {
		t.Error("Channel index for connection B should still have 1 consumer")
	}
}

func TestRegisterConsumer_GeneratesTagWhenEmpty(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Create consumer with empty tag
	consumer := NewConsumer(conn, 1, "test-queue", "", &ConsumerProperties{
		NoAck:     false,
		Exclusive: false,
		Arguments: nil,
	})

	// Initially the tag should be empty
	if consumer.Tag != "" {
		t.Errorf("Expected empty tag initially, got '%s'", consumer.Tag)
	}

	// Register the consumer - this should generate a unique tag
	err := vh.RegisterConsumer(consumer)
	if err != nil {
		t.Fatalf("Failed to register consumer: %v", err)
	}

	// After registration, the tag should be generated
	if consumer.Tag == "" {
		t.Error("Consumer tag should have been generated during registration")
	}

	// Tag should follow the expected format (amq.ctag-xxxxx)
	if len(consumer.Tag) < 15 || !strings.HasPrefix(consumer.Tag, "amq.ctag-") {
		t.Errorf("Generated consumer tag doesn't match expected format: '%s'", consumer.Tag)
	}

	// Verify consumer is properly registered
	key := ConsumerKey{Channel: 1, Tag: consumer.Tag}
	if _, exists := vh.Consumers[key]; !exists {
		t.Error("Consumer should exist in registry with generated tag")
	}
}

func TestNewConsumer_WithTag(t *testing.T) {
	conn := NewMockConnection("test-conn")

	consumer := NewConsumer(conn, 2, "my-queue", "my-consumer-tag", &ConsumerProperties{
		NoAck:     true,
		Exclusive: true,
		Arguments: map[string]any{"key": "value"},
	})

	if consumer.Tag != "my-consumer-tag" {
		t.Errorf("Expected tag 'my-consumer-tag', got '%s'", consumer.Tag)
	}

	if consumer.Channel != 2 {
		t.Errorf("Expected channel 2, got %d", consumer.Channel)
	}

	if consumer.QueueName != "my-queue" {
		t.Errorf("Expected queue name 'my-queue', got '%s'", consumer.QueueName)
	}

	if consumer.Connection != conn {
		t.Error("Consumer connection should match provided connection")
	}

	if consumer.Props == nil {
		t.Error("Consumer properties should not be nil")
	} else {
		if !consumer.Props.NoAck {
			t.Error("Consumer NoAck should be true")
		}

		if !consumer.Props.Exclusive {
			t.Error("Consumer Exclusive should be true")
		}

		if consumer.Props.Arguments == nil {
			t.Error("Consumer arguments should not be nil")
		} else if consumer.Props.Arguments["key"] != "value" {
			t.Error("Consumer arguments not preserved correctly")
		}
	}
}

func TestRegisterConsumer_EmptyTagUniqueness(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register multiple consumers with empty tags on the same channel
	consumer1 := NewConsumer(conn, 1, "test-queue", "", &ConsumerProperties{NoAck: false})
	consumer2 := NewConsumer(conn, 1, "test-queue", "", &ConsumerProperties{NoAck: false})
	consumer3 := NewConsumer(conn, 1, "test-queue", "", &ConsumerProperties{NoAck: false})

	// Register all consumers
	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register consumer1: %v", err)
	}

	err = vh.RegisterConsumer(consumer2)
	if err != nil {
		t.Fatalf("Failed to register consumer2: %v", err)
	}

	err = vh.RegisterConsumer(consumer3)
	if err != nil {
		t.Fatalf("Failed to register consumer3: %v", err)
	}

	// All tags should be different
	if consumer1.Tag == consumer2.Tag {
		t.Errorf("Consumer1 and Consumer2 should have different tags: '%s' vs '%s'", consumer1.Tag, consumer2.Tag)
	}

	if consumer1.Tag == consumer3.Tag {
		t.Errorf("Consumer1 and Consumer3 should have different tags: '%s' vs '%s'", consumer1.Tag, consumer3.Tag)
	}

	if consumer2.Tag == consumer3.Tag {
		t.Errorf("Consumer2 and Consumer3 should have different tags: '%s' vs '%s'", consumer2.Tag, consumer3.Tag)
	}

	// All should be registered
	if len(vh.Consumers) != 3 {
		t.Errorf("Expected 3 consumers registered, got %d", len(vh.Consumers))
	}

	// Verify each consumer is accessible by its generated tag
	key1 := ConsumerKey{Channel: 1, Tag: consumer1.Tag}
	key2 := ConsumerKey{Channel: 1, Tag: consumer2.Tag}
	key3 := ConsumerKey{Channel: 1, Tag: consumer3.Tag}

	if _, exists := vh.Consumers[key1]; !exists {
		t.Error("Consumer1 should be accessible by its generated tag")
	}
	if _, exists := vh.Consumers[key2]; !exists {
		t.Error("Consumer2 should be accessible by its generated tag")
	}
	if _, exists := vh.Consumers[key3]; !exists {
		t.Error("Consumer3 should be accessible by its generated tag")
	}
}

func TestRegisterConsumer_EmptyTagRetryLimit(t *testing.T) {
	// This test verifies that the retry mechanism has proper bounds
	// We can't easily test the actual retry limit without mocking the random generator,
	// but we can verify that normal operation works
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Create and register a consumer with empty tag
	consumer := NewConsumer(conn, 1, "test-queue", "", &ConsumerProperties{NoAck: false})

	err := vh.RegisterConsumer(consumer)
	if err != nil {
		t.Fatalf("Registration should succeed with retry mechanism: %v", err)
	}

	// Verify tag was generated
	if consumer.Tag == "" {
		t.Error("Tag should have been generated")
	}

	// Verify expected format
	if !strings.HasPrefix(consumer.Tag, "amq.ctag-") {
		t.Errorf("Generated tag should start with 'amq.ctag-', got: %s", consumer.Tag)
	}
}

// Tests for delivery functionality
func TestConsumerRegistrationAndCancellation(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Create consumer
	consumer := NewConsumer(conn, 1, "test-queue", "test-consumer", &ConsumerProperties{NoAck: false})

	// Test successful registration
	err := vh.RegisterConsumer(consumer)
	if err != nil {
		t.Fatalf("Failed to register consumer: %v", err)
	}

	// Verify consumer is registered
	if len(vh.Consumers) != 1 {
		t.Fatalf("Expected 1 consumer, got %d", len(vh.Consumers))
	}

	key := ConsumerKey{Channel: 1, Tag: "test-consumer"}
	if _, exists := vh.Consumers[key]; !exists {
		t.Error("Consumer should exist in consumers map")
	}

	if len(vh.ConsumersByQueue["test-queue"]) != 1 {
		t.Fatalf("Expected 1 consumer in round-robin list, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	if vh.ConsumersByQueue["test-queue"][0] != consumer {
		t.Error("Consumer should be in round-robin list")
	}

	// Test successful cancellation
	err = vh.CancelConsumer(1, "test-consumer")
	if err != nil {
		t.Fatalf("Failed to cancel consumer: %v", err)
	}

	// Verify consumer is removed
	if len(vh.Consumers) != 0 {
		t.Fatalf("Expected 0 consumers after cancellation, got %d", len(vh.Consumers))
	}

	if len(vh.ConsumersByQueue["test-queue"]) != 0 {
		t.Fatalf("Expected 0 consumers in round-robin list after cancellation, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	// Test cancellation of non-existent consumer
	err = vh.CancelConsumer(1, "non-existent")
	if err == nil {
		t.Error("Expected error when cancelling non-existent consumer")
	}

	expectedError := "consumer with tag non-existent on channel 1 does not exist"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExclusiveConsumerLogic(t *testing.T) {
	vh := createTestVHost()
	conn1 := NewMockConnection("conn1")
	conn2 := NewMockConnection("conn2")

	// Register first consumer as exclusive
	consumer1 := NewConsumer(conn1, 1, "test-queue", "exclusive-consumer", &ConsumerProperties{
		NoAck:     false,
		Exclusive: true,
	})
	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register exclusive consumer: %v", err)
	}

	// Verify exclusive consumer is registered
	if len(vh.Consumers) != 1 {
		t.Fatalf("Expected 1 consumer, got %d", len(vh.Consumers))
	}

	// Try to register second consumer (should fail due to exclusive)
	consumer2 := NewConsumer(conn2, 1, "test-queue", "second-consumer", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer2)
	if err == nil {
		t.Error("Expected error when registering second consumer with exclusive consumer present")
	}

	expectedError := "exclusive consumer already exists for queue test-queue"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Verify only the exclusive consumer exists
	if len(vh.Consumers) != 1 {
		t.Fatalf("Expected 1 consumer after failed registration, got %d", len(vh.Consumers))
	}

	if len(vh.ConsumersByQueue["test-queue"]) != 1 {
		t.Fatalf("Expected 1 consumer in round-robin list, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	// Cancel exclusive consumer
	err = vh.CancelConsumer(1, "exclusive-consumer")
	if err != nil {
		t.Fatalf("Failed to cancel exclusive consumer: %v", err)
	}

	// Now should be able to register new consumer
	consumer3 := NewConsumer(conn2, 1, "test-queue", "new-consumer", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer3)
	if err != nil {
		t.Fatalf("Failed to register consumer after exclusive removed: %v", err)
	}

	// Verify new consumer is registered
	if len(vh.Consumers) != 1 {
		t.Fatalf("Expected 1 consumer after new registration, got %d", len(vh.Consumers))
	}

	if vh.ConsumersByQueue["test-queue"][0] != consumer3 {
		t.Error("New consumer should be in round-robin list")
	}

	// Test that regular consumers can coexist
	consumer4 := NewConsumer(conn1, 2, "test-queue", "third-consumer", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer4)
	if err != nil {
		t.Fatalf("Failed to register third consumer: %v", err)
	}

	// Verify both non-exclusive consumers exist
	if len(vh.Consumers) != 2 {
		t.Fatalf("Expected 2 consumers, got %d", len(vh.Consumers))
	}

	if len(vh.ConsumersByQueue["test-queue"]) != 2 {
		t.Fatalf("Expected 2 consumers in round-robin list, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	// Now try to register exclusive consumer (should fail with existing consumers)
	consumer5 := NewConsumer(conn1, 3, "test-queue", "late-exclusive", &ConsumerProperties{
		NoAck:     false,
		Exclusive: true,
	})
	err = vh.RegisterConsumer(consumer5)
	if err == nil {
		t.Error("Expected error when registering exclusive consumer with existing consumers")
	}

	expectedError = "cannot add exclusive consumer when other consumers exist for queue test-queue"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestConsumerCleanupOnConnectionClose(t *testing.T) {
	vh := createTestVHost()
	conn1 := NewMockConnection("conn1")
	conn2 := NewMockConnection("conn2")

	// Register consumers from both connections
	consumer1 := NewConsumer(conn1, 1, "test-queue", "consumer1", &ConsumerProperties{NoAck: false})
	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register consumer1: %v", err)
	}

	consumer2 := NewConsumer(conn1, 2, "test-queue", "consumer2", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer2)
	if err != nil {
		t.Fatalf("Failed to register consumer2: %v", err)
	}

	consumer3 := NewConsumer(conn2, 1, "test-queue", "consumer3", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer3)
	if err != nil {
		t.Fatalf("Failed to register consumer3: %v", err)
	}

	// Verify all consumers are registered
	if len(vh.Consumers) != 3 {
		t.Fatalf("Expected 3 consumers, got %d", len(vh.Consumers))
	}

	if len(vh.ConsumersByQueue["test-queue"]) != 3 {
		t.Fatalf("Expected 3 consumers in round-robin list, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	// Clean up conn1 consumers
	vh.CleanupConnection(conn1)

	// Verify only conn2 consumer remains
	if len(vh.Consumers) != 1 {
		t.Fatalf("Expected 1 consumer after cleanup, got %d", len(vh.Consumers))
	}

	if len(vh.ConsumersByQueue["test-queue"]) != 1 {
		t.Fatalf("Expected 1 consumer in round-robin list after cleanup, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	// Verify the remaining consumer is from conn2
	key := ConsumerKey{Channel: 1, Tag: "consumer3"}
	if _, exists := vh.Consumers[key]; !exists {
		t.Error("Consumer3 from conn2 should still exist")
	}

	if vh.ConsumersByQueue["test-queue"][0] != consumer3 {
		t.Error("Consumer3 should be in round-robin list")
	}

	// Verify conn1 consumers are gone
	key1 := ConsumerKey{Channel: 1, Tag: "consumer1"}
	key2 := ConsumerKey{Channel: 2, Tag: "consumer2"}

	if _, exists := vh.Consumers[key1]; exists {
		t.Error("Consumer1 should be cleaned up")
	}

	if _, exists := vh.Consumers[key2]; exists {
		t.Error("Consumer2 should be cleaned up")
	}

	// Verify consumer1 and consumer2 are not in round-robin list
	for _, consumer := range vh.ConsumersByQueue["test-queue"] {
		if consumer == consumer1 || consumer == consumer2 {
			t.Error("Cleaned up consumers should not be in round-robin list")
		}
	}
}

func TestConsumerChannelCleanup(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Register consumers on different channels
	consumer1 := NewConsumer(conn, 1, "test-queue", "consumer1", &ConsumerProperties{NoAck: false})
	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register consumer1: %v", err)
	}

	consumer2 := NewConsumer(conn, 2, "test-queue", "consumer2", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer2)
	if err != nil {
		t.Fatalf("Failed to register consumer2: %v", err)
	}

	// Verify both consumers exist
	if len(vh.Consumers) != 2 {
		t.Fatalf("Expected 2 consumers, got %d", len(vh.Consumers))
	}

	// Clean up only channel 1
	vh.CleanupChannel(conn, 1)

	// Verify only consumer2 remains
	if len(vh.Consumers) != 1 {
		t.Fatalf("Expected 1 consumer after channel cleanup, got %d", len(vh.Consumers))
	}

	if len(vh.ConsumersByQueue["test-queue"]) != 1 {
		t.Fatalf("Expected 1 consumer in round-robin list after cleanup, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	// Verify consumer2 still exists
	key2 := ConsumerKey{Channel: 2, Tag: "consumer2"}
	if _, exists := vh.Consumers[key2]; !exists {
		t.Error("Consumer2 should still exist after channel 1 cleanup")
	}

	if vh.ConsumersByQueue["test-queue"][0] != consumer2 {
		t.Error("Consumer2 should be in round-robin list")
	}

	// Verify consumer1 is gone
	key1 := ConsumerKey{Channel: 1, Tag: "consumer1"}
	if _, exists := vh.Consumers[key1]; exists {
		t.Error("Consumer1 should be cleaned up")
	}

	// Verify consumer1 is not in round-robin list
	for _, consumer := range vh.ConsumersByQueue["test-queue"] {
		if consumer == consumer1 {
			t.Error("Consumer1 should not be in round-robin list after cleanup")
		}
	}
}

func TestRoundRobinConsumerRotation(t *testing.T) {
	vh := createTestVHost()
	conn1 := NewMockConnection("conn1")
	conn2 := NewMockConnection("conn2")
	conn3 := NewMockConnection("conn3")

	// Register three consumers
	consumer1 := NewConsumer(conn1, 1, "test-queue", "consumer1", &ConsumerProperties{NoAck: false})
	err := vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register consumer1: %v", err)
	}

	consumer2 := NewConsumer(conn2, 1, "test-queue", "consumer2", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer2)
	if err != nil {
		t.Fatalf("Failed to register consumer2: %v", err)
	}

	consumer3 := NewConsumer(conn3, 1, "test-queue", "consumer3", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer3)
	if err != nil {
		t.Fatalf("Failed to register consumer3: %v", err)
	}

	// Verify consumers are registered in order
	if len(vh.ConsumersByQueue["test-queue"]) != 3 {
		t.Fatalf("Expected 3 consumers, got %d", len(vh.ConsumersByQueue["test-queue"]))
	}

	// Verify initial round-robin order
	expectedOrder := []string{consumer1.Tag, consumer2.Tag, consumer3.Tag}
	for i, consumer := range vh.ConsumersByQueue["test-queue"] {
		if consumer.Tag != expectedOrder[i] {
			t.Errorf("Initial round-robin order incorrect at index %d: expected %s, got %s",
				i, expectedOrder[i], consumer.Tag)
		}
	}

	// Test manual round-robin rotation as done in delivery loop
	// Simulate the rotation that happens during message delivery
	vh.mu.Lock()
	consumers := vh.ConsumersByQueue["test-queue"]
	if len(consumers) > 0 {
		// Take first consumer and move to end (round-robin)
		firstConsumer := consumers[0]
		vh.ConsumersByQueue["test-queue"] = append(consumers[1:], firstConsumer)
	}
	vh.mu.Unlock()

	// After rotation, order should be [consumer2, consumer3, consumer1]
	rotatedOrder := []string{consumer2.Tag, consumer3.Tag, consumer1.Tag}
	for i, consumer := range vh.ConsumersByQueue["test-queue"] {
		if consumer.Tag != rotatedOrder[i] {
			t.Errorf("Round-robin rotation incorrect at index %d: expected %s, got %s",
				i, rotatedOrder[i], consumer.Tag)
		}
	}

	// Test another rotation
	vh.mu.Lock()
	consumers = vh.ConsumersByQueue["test-queue"]
	if len(consumers) > 0 {
		firstConsumer := consumers[0]
		vh.ConsumersByQueue["test-queue"] = append(consumers[1:], firstConsumer)
	}
	vh.mu.Unlock()

	// After second rotation, order should be [consumer3, consumer1, consumer2]
	secondRotatedOrder := []string{consumer3.Tag, consumer1.Tag, consumer2.Tag}
	for i, consumer := range vh.ConsumersByQueue["test-queue"] {
		if consumer.Tag != secondRotatedOrder[i] {
			t.Errorf("Second round-robin rotation incorrect at index %d: expected %s, got %s",
				i, secondRotatedOrder[i], consumer.Tag)
		}
	}
}

func TestConsumerEdgeCases(t *testing.T) {
	vh := createTestVHost()
	conn := NewMockConnection("test-conn")

	// Test getting consumers for non-existent queue
	consumers := vh.GetActiveConsumersForQueue("non-existent-queue")
	if consumers != nil {
		t.Error("Expected nil for non-existent queue consumers")
	}

	// Test getting consumers for empty queue
	consumers = vh.GetActiveConsumersForQueue("test-queue")
	if len(consumers) != 0 {
		t.Errorf("Expected 0 consumers for empty queue, got %d", len(consumers))
	}

	// Test registering consumer for non-existent queue
	consumer := NewConsumer(conn, 1, "non-existent-queue", "test-consumer", &ConsumerProperties{NoAck: false})
	err := vh.RegisterConsumer(consumer)
	if err == nil {
		t.Error("Expected error when registering consumer for non-existent queue")
	}

	expectedError := "queue non-existent-queue does not exist"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Test duplicate consumer registration
	consumer1 := NewConsumer(conn, 1, "test-queue", "duplicate-tag", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer1)
	if err != nil {
		t.Fatalf("Failed to register first consumer: %v", err)
	}

	consumer2 := NewConsumer(conn, 1, "test-queue", "duplicate-tag", &ConsumerProperties{NoAck: false})
	err = vh.RegisterConsumer(consumer2)
	if err == nil {
		t.Error("Expected error when registering duplicate consumer")
	}

	expectedError = "consumer with tag duplicate-tag already exists on channel 1"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}
