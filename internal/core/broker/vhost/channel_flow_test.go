package vhost

import (
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

// Test Suite: Channel Flow Control (AMQP channel.flow)
//
// This test suite validates the implementation of AMQP 0.9.1 channel.flow mechanism.
//
// Key concepts tested:
// - Client-initiated flow: Client pauses/resumes message delivery to itself
// - Server-initiated flow: Server pauses/resumes message publishing by client
// - Flow state affects shouldThrottle() delivery logic
// - Flow control is independent per channel (not global)
// - FlowActive defaults to true (flow enabled) for new channels
//
// AMQP Specification Notes:
// - channel.flow controls CONTENT frames (HEADER + BODY), not METHOD frames
// - active=true means "flow is ON" (normal operation)
// - active=false means "flow is OFF" (pause content delivery)
// - basic.get (synchronous pull) is NOT affected by flow control
// - basic.consume (asynchronous push) IS affected by flow control

// TestHandleChannelFlow_ClientInitiated tests client-initiated flow control
func TestHandleChannelFlow_ClientInitiated(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel := uint16(1)

	// Test 1: Client pauses flow (active=false)
	err := vh.HandleChannelFlow(conn, channel, false, false)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed: %v", err)
	}

	flowState := vh.GetChannelFlowState(conn, channel)
	if flowState.FlowActive {
		t.Error("Expected FlowActive to be false after client paused flow")
	}
	if flowState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be false for client-initiated flow")
	}

	// Test 2: Client resumes flow (active=true)
	err = vh.HandleChannelFlow(conn, channel, true, false)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed: %v", err)
	}

	flowState = vh.GetChannelFlowState(conn, channel)
	if !flowState.FlowActive {
		t.Error("Expected FlowActive to be true after client resumed flow")
	}
	if flowState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be false for client-initiated flow")
	}
}

// TestHandleChannelFlow_ServerInitiated tests server-initiated flow control
func TestHandleChannelFlow_ServerInitiated(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel := uint16(1)

	// Test: Server pauses flow (active=false, initiated by broker)
	err := vh.HandleChannelFlow(conn, channel, false, true)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed: %v", err)
	}

	flowState := vh.GetChannelFlowState(conn, channel)
	if flowState.FlowActive {
		t.Error("Expected FlowActive to be false after server paused flow")
	}
	if !flowState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be true for server-initiated flow")
	}

	// Test: Server resumes flow
	err = vh.HandleChannelFlow(conn, channel, true, true)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed: %v", err)
	}

	flowState = vh.GetChannelFlowState(conn, channel)
	if !flowState.FlowActive {
		t.Error("Expected FlowActive to be true after server resumed flow")
	}
}

// TestGetChannelFlowState_NonExistentChannel tests default state for non-existent channel
func TestGetChannelFlowState_NonExistentChannel(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel := uint16(99)

	// Should return default state (active=true, not broker-initiated)
	flowState := vh.GetChannelFlowState(conn, channel)
	if !flowState.FlowActive {
		t.Error("Expected FlowActive to be true for non-existent channel (default)")
	}
	if flowState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be false for non-existent channel (default)")
	}
}

// TestGetOrCreateChannelDelivery_DefaultFlowState tests that new channels start with flow active
func TestGetOrCreateChannelDelivery_DefaultFlowState(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel := uint16(1)

	channelKey := ConnectionChannelKey{conn, channel}
	channelState := vh.GetOrCreateChannelDelivery(channelKey)

	if !channelState.FlowActive {
		t.Error("Expected FlowActive to be true by default for new channel")
	}
	if channelState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be false by default for new channel")
	}
}

// TestShouldThrottle_FlowPaused tests that shouldThrottle returns true when flow is paused
func TestShouldThrottle_FlowPaused(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel := uint16(1)

	// Create a consumer
	consumer := &Consumer{
		Tag:        "test-consumer",
		Channel:    channel,
		QueueName:  "test-queue",
		Connection: conn,
		Active:     true,
		Props: &ConsumerProperties{
			NoAck: false,
		},
		PrefetchCount: 0, // No QoS limit
	}

	// Get channel state and pause flow
	channelKey := ConnectionChannelKey{conn, channel}
	channelState := vh.GetOrCreateChannelDelivery(channelKey)
	channelState.FlowActive = false

	// Should throttle due to paused flow
	if !vh.shouldThrottle(consumer, channelState) {
		t.Error("Expected shouldThrottle to return true when flow is paused")
	}

	// Resume flow
	channelState.FlowActive = true

	// Should NOT throttle (no QoS limits)
	if vh.shouldThrottle(consumer, channelState) {
		t.Error("Expected shouldThrottle to return false when flow is active and no QoS")
	}
}

// TestShouldThrottle_FlowAndQoS tests interaction between flow control and QoS
func TestShouldThrottle_FlowAndQoS(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel := uint16(1)

	consumer := &Consumer{
		Tag:           "test-consumer",
		Channel:       channel,
		QueueName:     "test-queue",
		Connection:    conn,
		Active:        true,
		Props:         &ConsumerProperties{NoAck: false},
		PrefetchCount: 5, // QoS limit
	}

	channelKey := ConnectionChannelKey{conn, channel}
	channelState := vh.GetOrCreateChannelDelivery(channelKey)

	// Test 1: Flow active, no unacked messages - should NOT throttle
	if vh.shouldThrottle(consumer, channelState) {
		t.Error("Expected no throttling with active flow and no unacked messages")
	}

	// Test 2: Flow paused, no unacked messages - should throttle due to flow
	channelState.FlowActive = false
	if !vh.shouldThrottle(consumer, channelState) {
		t.Error("Expected throttling when flow is paused")
	}

	// Test 3: Flow active, QoS limit reached - should throttle due to QoS
	channelState.FlowActive = true
	channelState.mu.Lock()
	for i := uint64(1); i <= 5; i++ {
		channelState.Unacked[i] = &DeliveryRecord{
			DeliveryTag: i,
			ConsumerTag: consumer.Tag,
			QueueName:   consumer.QueueName,
			Message:     Message{ID: "test"},
		}
	}
	channelState.mu.Unlock()

	if !vh.shouldThrottle(consumer, channelState) {
		t.Error("Expected throttling when QoS limit is reached")
	}

	// Test 4: Flow paused AND QoS limit reached - should still throttle
	channelState.FlowActive = false
	if !vh.shouldThrottle(consumer, channelState) {
		t.Error("Expected throttling when both flow is paused and QoS limit reached")
	}
}

// TestHandleChannelFlow_MultipleChannels tests flow control on different channels
func TestHandleChannelFlow_MultipleChannels(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel1 := uint16(1)
	channel2 := uint16(2)

	// Pause flow on channel 1
	err := vh.HandleChannelFlow(conn, channel1, false, false)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed for channel 1: %v", err)
	}

	// Channel 2 should still have active flow (independent)
	flowState1 := vh.GetChannelFlowState(conn, channel1)
	flowState2 := vh.GetChannelFlowState(conn, channel2)

	if flowState1.FlowActive {
		t.Error("Expected channel 1 flow to be paused")
	}
	if !flowState2.FlowActive {
		t.Error("Expected channel 2 flow to remain active (default)")
	}

	// Pause flow on channel 2
	err = vh.HandleChannelFlow(conn, channel2, false, false)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed for channel 2: %v", err)
	}

	flowState2 = vh.GetChannelFlowState(conn, channel2)
	if flowState2.FlowActive {
		t.Error("Expected channel 2 flow to be paused")
	}

	// Resume flow on channel 1 only
	err = vh.HandleChannelFlow(conn, channel1, true, false)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed for channel 1: %v", err)
	}

	flowState1 = vh.GetChannelFlowState(conn, channel1)
	flowState2 = vh.GetChannelFlowState(conn, channel2)

	if !flowState1.FlowActive {
		t.Error("Expected channel 1 flow to be active")
	}
	if flowState2.FlowActive {
		t.Error("Expected channel 2 flow to remain paused")
	}
}

// TestHandleChannelFlow_OverrideInitiator tests that new flow requests override previous initiator
func TestHandleChannelFlow_OverrideInitiator(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	channel := uint16(1)

	// Server initiates flow pause
	err := vh.HandleChannelFlow(conn, channel, false, true)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed: %v", err)
	}

	flowState := vh.GetChannelFlowState(conn, channel)
	if !flowState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be true")
	}

	// Client requests flow resume (overrides server's state)
	err = vh.HandleChannelFlow(conn, channel, true, false)
	if err != nil {
		t.Fatalf("HandleChannelFlow failed: %v", err)
	}

	flowState = vh.GetChannelFlowState(conn, channel)
	if !flowState.FlowActive {
		t.Error("Expected FlowActive to be true")
	}
	if flowState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be false after client override")
	}
}

// TestShouldThrottle_NilChannelState tests that nil channel state doesn't cause panic
func TestShouldThrottle_NilChannelState(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil

	consumer := &Consumer{
		Tag:        "test-consumer",
		Channel:    1,
		QueueName:  "test-queue",
		Connection: conn,
		Active:     true,
		Props:      &ConsumerProperties{NoAck: false},
	}

	// Should not throttle with nil channel state (no QoS, flow defaults to active)
	if vh.shouldThrottle(consumer, nil) {
		t.Error("Expected shouldThrottle to return false with nil channel state")
	}
}
