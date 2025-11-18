package vhost

import (
	"net"
	"testing"
)

// helper to build a minimal consumer
func newTestConsumer(conn net.Conn, ch uint16, queue string, noAck bool) *Consumer {
	return &Consumer{
		Tag:        "ctag-test",
		Channel:    ch,
		QueueName:  queue,
		Connection: conn,
		Active:     true,
		Props: &ConsumerProperties{
			NoAck: noAck,
		},
	}
}

func TestChannelDeliveryState_SingleAck(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)

	// Fake connection (nil is sufficient for HandleBasicAck)
	var conn net.Conn = nil

	// Register consumer (manual ack)
	c := newTestConsumer(conn, 1, "q1", false)

	// Manually simulate two deliveries by recording unacked entries
	key := ConnectionChannelKey{conn, c.Channel}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Simulate two delivered messages (manual ack)
	m1 := Message{ID: "m1", Body: []byte("m1")}
	m2 := Message{ID: "m2", Body: []byte("m2")}

	ch.mu.Lock()
	ch.LastDeliveryTag++
	t1 := ch.LastDeliveryTag
	record := &DeliveryRecord{DeliveryTag: t1, ConsumerTag: c.Tag, QueueName: c.QueueName, Message: m1}
	ch.UnackedByTag[t1] = record
	if ch.UnackedByConsumer[c.Tag] == nil {
		ch.UnackedByConsumer[c.Tag] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer[c.Tag][t1] = record

	ch.LastDeliveryTag++
	t2 := ch.LastDeliveryTag
	record = &DeliveryRecord{DeliveryTag: t2, ConsumerTag: c.Tag, QueueName: c.QueueName, Message: m2}
	ch.UnackedByTag[t2] = record
	if ch.UnackedByConsumer[c.Tag] == nil {
		ch.UnackedByConsumer[c.Tag] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer[c.Tag][t2] = record
	ch.mu.Unlock()

	if len(ch.UnackedByTag) != 2 {
		t.Fatalf("expected 2 unacked, got %d", len(ch.UnackedByTag))
	}

	// Ack first only (single)
	if err := vh.HandleBasicAck(conn, c.Channel, t1, false); err != nil {
		t.Fatalf("HandleBasicAck error: %v", err)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()
	if _, ok := ch.UnackedByTag[t1]; ok {
		t.Fatalf("delivery tag %d should be removed after ack", t1)
	}
	if _, ok := ch.UnackedByTag[t2]; !ok {
		t.Fatalf("delivery tag %d should remain unacked", t2)
	}
}

func TestChannelDeliveryState_MultipleAck(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	var conn net.Conn = nil
	key := ConnectionChannelKey{conn, 1}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}

	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()
	// Register consumer
	c := newTestConsumer(conn, 1, "q-multi", false)
	// Simulate deliveries to track unacked messages

	// create 3 tags
	ch.mu.Lock()
	for i := 0; i < 3; i++ {
		ch.LastDeliveryTag++
		tag := ch.LastDeliveryTag
		record := &DeliveryRecord{DeliveryTag: tag}
		ch.UnackedByTag[tag] = record
		if ch.UnackedByConsumer[c.Tag] == nil {
			ch.UnackedByConsumer[c.Tag] = make(map[uint64]*DeliveryRecord)
		}
	}
	ch.mu.Unlock()

	if len(ch.UnackedByTag) != 3 {
		t.Fatalf("expected 3 unacked, got %d", len(ch.UnackedByTag))
	}

	// multiple ack up to second tag
	upTo := uint64(2)
	if err := vh.HandleBasicAck(conn, 1, upTo, true); err != nil {
		t.Fatalf("HandleBasicAck error: %v", err)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()
	if len(ch.UnackedByTag) != 1 {
		t.Fatalf("expected 1 unacked remaining, got %d", len(ch.UnackedByTag))
	}
	if _, ok := ch.UnackedByTag[3]; !ok {
		t.Fatalf("expected tag 3 to remain")
	}
}

func TestChannelDeliveryState_UnknownTag(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	var conn net.Conn = nil
	// Ack when there is no state — should return error in current implementation
	if err := vh.HandleBasicAck(conn, 1, 42, false); err == nil {
		t.Fatalf("expected error when channel delivery state is missing")
	}
}

func TestDeliverTracking_NoAckFlag(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	var conn net.Conn = nil
	key := ConnectionChannelKey{conn, 2}

	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Register consumer
	c := newTestConsumer(conn, 2, "q-noack", false)

	// Simulate manual-ack delivery — stored
	ch.mu.Lock()
	ch.LastDeliveryTag++
	tag1 := ch.LastDeliveryTag
	// ch.Unacked[tag1] = &DeliveryRecord{DeliveryTag: tag1}
	ch.UnackedByTag[tag1] = &DeliveryRecord{DeliveryTag: tag1}
	if ch.UnackedByConsumer[c.Tag] == nil {
		ch.UnackedByConsumer[c.Tag] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer[c.Tag][tag1] = ch.UnackedByTag[tag1]
	ch.mu.Unlock()

	// Simulate auto-ack delivery — not stored
	ch.mu.Lock()
	ch.LastDeliveryTag++
	// intentionally do NOT store second tag to simulate NoAck=true
	ch.mu.Unlock()

	if len(ch.UnackedByTag) != 1 {
		t.Fatalf("expected only 1 unacked stored (manual ack), got %d", len(ch.UnackedByTag))
	}
}

func TestCleanupChannel_RequeuesUnacked(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("test-vhost", options)
	var conn net.Conn = nil
	// Create a queue
	q, err := vh.CreateQueue("q-clean", nil, conn)
	if err != nil {
		t.Fatalf("CreateQueue error: %v", err)
	}

	// Prepare an unacked record on channel (conn=nil, ch=3)
	key := ConnectionChannelKey{conn, 3}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Register consumer
	c := newTestConsumer(conn, 3, q.Name, false)

	msg := Message{ID: "mx", Body: []byte("x")}
	ch.mu.Lock()
	ch.LastDeliveryTag = 1
	record := &DeliveryRecord{DeliveryTag: 1, QueueName: q.Name, Message: msg}
	ch.UnackedByTag[1] = record
	if ch.UnackedByConsumer[c.Tag] == nil {
		ch.UnackedByConsumer[c.Tag] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer[c.Tag][1] = record
	ch.mu.Unlock()

	// Cleanup should requeue the unacked message into q
	vh.CleanupChannel(conn, 3)

	if got := q.Len(); got != 1 {
		t.Fatalf("expected queue length 1 after requeue, got %d", got)
	}

	// ChannelDeliveries entry should be removed
	vh.mu.Lock()
	_, exists := vh.ChannelDeliveries[key]
	vh.mu.Unlock()
	if exists {
		t.Fatalf("expected ChannelDeliveries entry to be removed after cleanup")
	}
}
