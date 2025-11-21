package vhost

import (
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/testutil"
)

// mockFrameSender implements FrameSender for testing
type mockFrameSender struct {
	sentFrames []sentFrame
	sendError  error
}

type sentFrame struct {
	connID  ConnectionID
	channel uint16
	frame   []byte
}

func (m *mockFrameSender) SendFrame(connID ConnectionID, channel uint16, frame []byte) error {
	if m.sendError != nil {
		return m.sendError
	}
	m.sentFrames = append(m.sentFrames, sentFrame{
		connID:  connID,
		channel: channel,
		frame:   frame,
	})
	return nil
}

func TestHandleBasicRecover_RequeueTrue(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	connID := newTestConsumerConnID()

	// Create queue
	q, err := vh.CreateQueue("q1", nil, connID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Setup channel delivery state with unacked messages
	key := ConnectionChannelKey{connID, 1}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Add unacked messages
	m1 := Message{ID: "msg1", Body: []byte("test1")}
	m2 := Message{ID: "msg2", Body: []byte("test2")}

	ch.mu.Lock()
	record := &DeliveryRecord{
		DeliveryTag: 1,
		ConsumerTag: "ctag1",
		QueueName:   "q1",
		Message:     m1,
	}
	ch.UnackedByTag[1] = record
	if ch.UnackedByConsumer["ctag1"] == nil {
		ch.UnackedByConsumer["ctag1"] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer["ctag1"][1] = record

	record2 := &DeliveryRecord{
		DeliveryTag: 2,
		ConsumerTag: "ctag1",
		QueueName:   "q1",
		Message:     m2,
	}
	ch.UnackedByTag[2] = record2
	if ch.UnackedByConsumer["ctag1"] == nil {
		ch.UnackedByConsumer["ctag1"] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer["ctag1"][2] = record2
	ch.mu.Unlock()

	// Call recover with requeue=true
	if err := vh.HandleBasicRecover(connID, 1, true); err != nil {
		t.Fatalf("HandleBasicRecover failed: %v", err)
	}

	// Verify unacked is cleared
	ch.mu.Lock()
	unackedCount := len(ch.UnackedByTag)
	ch.mu.Unlock()
	if unackedCount != 0 {
		t.Errorf("expected 0 unacked after recover, got %d", unackedCount)
	}

	// Verify messages requeued
	if q.Len() != 2 {
		t.Errorf("expected 2 messages in queue, got %d", q.Len())
	}

	// Verify messages marked as redelivered
	if !vh.ShouldRedeliver("msg1") {
		t.Error("msg1 should be marked for redelivery")
	}
	if !vh.ShouldRedeliver("msg2") {
		t.Error("msg2 should be marked for redelivery")
	}
}

func TestHandleBasicRecover_RequeueFalse_ConsumerExists(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	connID := newTestConsumerConnID()

	// Create queue
	if _, err := vh.CreateQueue("q1", nil, connID); err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Register consumer
	consumer := &Consumer{
		Tag:          "ctag1",
		Channel:      1,
		QueueName:    "q1",
		ConnectionID: connID,
		Active:       true,
		Props:        &ConsumerProperties{NoAck: false},
	}

	consumerKey := ConsumerKey{Channel: 1, Tag: "ctag1"}
	vh.mu.Lock()
	vh.Consumers[consumerKey] = consumer
	channelKey := ConnectionChannelKey{connID, 1}
	vh.ConsumersByChannel[channelKey] = []*Consumer{consumer}
	vh.mu.Unlock()

	// Setup mock framer that always succeeds
	vh.framer = &testutil.MockFramer{}

	// Setup mock frame sender
	mockSender := &mockFrameSender{}
	vh.SetFrameSender(mockSender)

	// Setup channel delivery state with unacked message
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[channelKey] = ch
	vh.mu.Unlock()

	m1 := Message{ID: "msg1", Body: []byte("test1")}
	ch.mu.Lock()
	ch.LastDeliveryTag = 1 // Set counter so next delivery gets tag 2
	ch.UnackedByTag[1] = &DeliveryRecord{
		DeliveryTag: 1,
		ConsumerTag: "ctag1",
		QueueName:   "q1",
		Message:     m1,
	}
	if ch.UnackedByConsumer["ctag1"] == nil {
		ch.UnackedByConsumer["ctag1"] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer["ctag1"][1] = ch.UnackedByTag[1]
	ch.mu.Unlock()

	// Call recover with requeue=false
	if err := vh.HandleBasicRecover(connID, 1, false); err != nil {
		t.Fatalf("HandleBasicRecover failed: %v", err)
	}

	// Verify new delivery was created with new tag (old tag 1 was cleared, new tag assigned)
	ch.mu.Lock()
	_, oldExists := ch.UnackedByTag[1]
	newUnackedCount := len(ch.UnackedByTag)
	var newTag uint64
	for tag := range ch.UnackedByTag {
		newTag = tag
		break
	}
	ch.mu.Unlock()

	if oldExists {
		t.Error("old delivery tag 1 should be cleared")
	}
	if newUnackedCount != 1 {
		t.Errorf("expected 1 new unacked after redelivery, got %d", newUnackedCount)
	}
	if newTag != 2 {
		t.Errorf("expected new delivery tag 2 (ch.LastDeliveryTag incremented from 1), got %d", newTag)
	}
}

func TestHandleBasicRecover_RequeueFalse_ConsumerGone(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	connID := newTestConsumerConnID()

	// Create queue
	q, err := vh.CreateQueue("q1", nil, connID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Setup channel delivery state with unacked message, but NO consumer
	key := ConnectionChannelKey{connID, 1}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	m1 := Message{ID: "msg1", Body: []byte("test1")}
	ch.mu.Lock()
	ch.UnackedByTag[1] = &DeliveryRecord{
		DeliveryTag: 1,
		ConsumerTag: "ctag-missing",
		QueueName:   "q1",
		Message:     m1,
	}
	if ch.UnackedByConsumer["ctag-missing"] == nil {
		ch.UnackedByConsumer["ctag-missing"] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer["ctag-missing"][1] = ch.UnackedByTag[1]
	ch.mu.Unlock()

	// Call recover with requeue=false
	if err := vh.HandleBasicRecover(connID, 1, false); err != nil {
		t.Fatalf("HandleBasicRecover failed: %v", err)
	}

	// Verify unacked is cleared
	ch.mu.Lock()
	unackedCount := len(ch.UnackedByTag)
	ch.mu.Unlock()
	if unackedCount != 0 {
		t.Errorf("expected 0 unacked after recover, got %d", unackedCount)
	}

	// Verify message requeued (fallback when consumer is gone)
	if q.Len() != 1 {
		t.Errorf("expected 1 message requeued, got %d", q.Len())
	}

	// Verify message marked as redelivered
	if !vh.ShouldRedeliver("msg1") {
		t.Error("msg1 should be marked for redelivery")
	}
}

func TestHandleBasicRecover_RequeueFalse_DeliveryFails(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	connID := newTestConsumerConnID()

	// Create queue
	q, err := vh.CreateQueue("q1", nil, connID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Register consumer
	consumer := &Consumer{
		Tag:          "ctag1",
		Channel:      1,
		QueueName:    "q1",
		ConnectionID: connID,
		Active:       true,
		Props:        &ConsumerProperties{NoAck: false},
	}

	consumerKey := ConsumerKey{Channel: 1, Tag: "ctag1"}
	vh.mu.Lock()
	vh.Consumers[consumerKey] = consumer
	channelKey := ConnectionChannelKey{connID, 1}
	vh.ConsumersByChannel[channelKey] = []*Consumer{consumer}
	vh.mu.Unlock()

	// Setup mock framer that FAILS
	vh.framer = &testutil.MockFramer{SendError: &net.OpError{Op: "write", Err: net.ErrClosed}}

	// Setup mock frame sender that FAILS
	mockSender := &mockFrameSender{sendError: &net.OpError{Op: "write", Err: net.ErrClosed}}
	vh.SetFrameSender(mockSender)

	// Setup channel delivery state with unacked message
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[channelKey] = ch
	vh.mu.Unlock()

	m1 := Message{ID: "msg1", Body: []byte("test1")}
	ch.mu.Lock()
	ch.UnackedByTag[1] = &DeliveryRecord{
		DeliveryTag: 1,
		ConsumerTag: "ctag1",
		QueueName:   "q1",
		Message:     m1,
	}
	if ch.UnackedByConsumer["ctag1"] == nil {
		ch.UnackedByConsumer["ctag1"] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer["ctag1"][1] = ch.UnackedByTag[1]

	ch.mu.Unlock()

	// Call recover with requeue=false
	if err := vh.HandleBasicRecover(connID, 1, false); err != nil {
		t.Fatalf("HandleBasicRecover failed: %v", err)
	}

	// Verify old unacked is cleared
	ch.mu.Lock()
	unackedCount := len(ch.UnackedByTag)
	ch.mu.Unlock()
	if unackedCount != 0 {
		t.Errorf("expected 0 unacked after failed redelivery, got %d", unackedCount)
	}

	// Verify message requeued (fallback on delivery failure)
	if q.Len() != 1 {
		t.Errorf("expected 1 message requeued after failed delivery, got %d", q.Len())
	}

	// Verify message marked as redelivered
	if !vh.ShouldRedeliver("msg1") {
		t.Error("msg1 should be marked for redelivery after failed delivery")
	}
}

func TestHandleBasicRecover_NoChannelState(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	connID := newTestConsumerConnID()

	// Call recover without setting up channel state
	err := vh.HandleBasicRecover(connID, 1, true)
	if err == nil {
		t.Error("expected error when channel state missing, got nil")
	}
}

func TestRedeliveredMarkLifecycle(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)

	// Mark a message as redelivered
	vh.markAsRedelivered("msg1")

	// Verify it's marked
	if !vh.ShouldRedeliver("msg1") {
		t.Error("msg1 should be marked for redelivery")
	}

	// Clear the mark
	vh.clearRedeliveredMark("msg1")

	// Verify it's cleared
	if vh.ShouldRedeliver("msg1") {
		t.Error("msg1 should not be marked after clearing")
	}
}

func TestHandleBasicRecover_RequeueFalse_BasicGetSentinel(t *testing.T) {
	// Test that Basic.Get deliveries (with BASIC_GET_SENTINEL) are requeued
	// when Basic.Recover(requeue=false) is called, since they have no consumer
	// to redeliver to.
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	connID := newTestConsumerConnID()

	// Create queue
	q, err := vh.CreateQueue("q1", nil, connID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// Setup channel delivery state with Basic.Get unacked message (using sentinel)
	key := ConnectionChannelKey{connID, 1}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	m1 := Message{ID: "msg1", Body: []byte("test1")}
	ch.mu.Lock()
	record := &DeliveryRecord{
		DeliveryTag: 1,
		ConsumerTag: BASIC_GET_SENTINEL, // Basic.Get uses sentinel
		QueueName:   "q1",
		Message:     m1,
	}
	ch.UnackedByTag[1] = record
	if ch.UnackedByConsumer[BASIC_GET_SENTINEL] == nil {
		ch.UnackedByConsumer[BASIC_GET_SENTINEL] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer[BASIC_GET_SENTINEL][1] = record
	ch.mu.Unlock()

	// Call recover with requeue=false
	// Since BASIC_GET_SENTINEL has no consumer, it should requeue the message
	if err := vh.HandleBasicRecover(connID, 1, false); err != nil {
		t.Fatalf("HandleBasicRecover failed: %v", err)
	}

	// Verify unacked is cleared
	ch.mu.Lock()
	unackedCount := len(ch.UnackedByTag)
	sentinelUnacked := len(ch.UnackedByConsumer[BASIC_GET_SENTINEL])
	ch.mu.Unlock()
	if unackedCount != 0 {
		t.Errorf("expected 0 unacked after recover, got %d", unackedCount)
	}
	if sentinelUnacked != 0 {
		t.Errorf("expected 0 sentinel unacked after recover, got %d", sentinelUnacked)
	}

	// Verify message was requeued (not redelivered, since Basic.Get has no consumer)
	if q.Len() != 1 {
		t.Errorf("expected 1 message requeued (Basic.Get has no consumer), got %d", q.Len())
	}

	// Verify message marked as redelivered
	if !vh.ShouldRedeliver("msg1") {
		t.Error("msg1 should be marked for redelivery")
	}
}
