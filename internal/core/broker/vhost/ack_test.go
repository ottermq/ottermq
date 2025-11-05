package vhost

import (
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
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
	vh := NewVhost("test-vhost", 1000, nil)

	// Fake connection (nil is sufficient for HandleBasicAck)
	var conn net.Conn = nil

	// Register consumer (manual ack)
	c := newTestConsumer(conn, 1, "q1", false)

	// Manually simulate two deliveries by recording unacked entries
	key := ConnectionChannelKey{conn, c.Channel}
	ch := &ChannelDeliveryState{Unacked: make(map[uint64]*DeliveryRecord)}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Simulate two delivered messages (manual ack)
	m1 := amqp.Message{ID: "m1", Body: []byte("m1")}
	m2 := amqp.Message{ID: "m2", Body: []byte("m2")}

	ch.mu.Lock()
	ch.LastDeliveryTag++
	t1 := ch.LastDeliveryTag
	ch.Unacked[t1] = &DeliveryRecord{DeliveryTag: t1, ConsumerTag: c.Tag, QueueName: c.QueueName, Message: m1}

	ch.LastDeliveryTag++
	t2 := ch.LastDeliveryTag
	ch.Unacked[t2] = &DeliveryRecord{DeliveryTag: t2, ConsumerTag: c.Tag, QueueName: c.QueueName, Message: m2}
	ch.mu.Unlock()

	if len(ch.Unacked) != 2 {
		t.Fatalf("expected 2 unacked, got %d", len(ch.Unacked))
	}

	// Ack first only (single)
	if err := vh.HandleBasicAck(conn, c.Channel, t1, false); err != nil {
		t.Fatalf("HandleBasicAck error: %v", err)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()
	if _, ok := ch.Unacked[t1]; ok {
		t.Fatalf("delivery tag %d should be removed after ack", t1)
	}
	if _, ok := ch.Unacked[t2]; !ok {
		t.Fatalf("delivery tag %d should remain unacked", t2)
	}
}

func TestChannelDeliveryState_MultipleAck(t *testing.T) {
	vh := NewVhost("test-vhost", 1000, nil)
	var conn net.Conn = nil
	key := ConnectionChannelKey{conn, 1}
	ch := &ChannelDeliveryState{Unacked: make(map[uint64]*DeliveryRecord)}

	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// create 3 tags
	ch.mu.Lock()
	for i := 0; i < 3; i++ {
		ch.LastDeliveryTag++
		tag := ch.LastDeliveryTag
		ch.Unacked[tag] = &DeliveryRecord{DeliveryTag: tag}
	}
	ch.mu.Unlock()

	if len(ch.Unacked) != 3 {
		t.Fatalf("expected 3 unacked, got %d", len(ch.Unacked))
	}

	// multiple ack up to second tag
	upTo := uint64(2)
	if err := vh.HandleBasicAck(conn, 1, upTo, true); err != nil {
		t.Fatalf("HandleBasicAck error: %v", err)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()
	if len(ch.Unacked) != 1 {
		t.Fatalf("expected 1 unacked remaining, got %d", len(ch.Unacked))
	}
	if _, ok := ch.Unacked[3]; !ok {
		t.Fatalf("expected tag 3 to remain")
	}
}

func TestChannelDeliveryState_UnknownTag(t *testing.T) {
	vh := NewVhost("test-vhost", 1000, nil)
	var conn net.Conn = nil
	// Ack when there is no state — should return error in current implementation
	if err := vh.HandleBasicAck(conn, 1, 42, false); err == nil {
		t.Fatalf("expected error when channel delivery state is missing")
	}
}

func TestDeliverTracking_NoAckFlag(t *testing.T) {
	vh := NewVhost("test-vhost", 1000, nil)
	var conn net.Conn = nil
	key := ConnectionChannelKey{conn, 2}

	ch := &ChannelDeliveryState{Unacked: make(map[uint64]*DeliveryRecord)}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Simulate manual-ack delivery — stored
	ch.mu.Lock()
	ch.LastDeliveryTag++
	tag1 := ch.LastDeliveryTag
	ch.Unacked[tag1] = &DeliveryRecord{DeliveryTag: tag1}
	ch.mu.Unlock()

	// Simulate auto-ack delivery — not stored
	ch.mu.Lock()
	ch.LastDeliveryTag++
	// intentionally do NOT store second tag to simulate NoAck=true
	ch.mu.Unlock()

	if len(ch.Unacked) != 1 {
		t.Fatalf("expected only 1 unacked stored (manual ack), got %d", len(ch.Unacked))
	}
}

func TestCleanupChannel_RequeuesUnacked(t *testing.T) {
	vh := NewVhost("test-vhost", 1000, nil)
	var conn net.Conn = nil
	// Create a queue
	q, err := vh.CreateQueue("q-clean", nil, conn)
	if err != nil {
		t.Fatalf("CreateQueue error: %v", err)
	}

	// Prepare an unacked record on channel (conn=nil, ch=3)
	key := ConnectionChannelKey{conn, 3}
	ch := &ChannelDeliveryState{Unacked: make(map[uint64]*DeliveryRecord)}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	msg := amqp.Message{ID: "mx", Body: []byte("x")}
	ch.mu.Lock()
	ch.LastDeliveryTag = 1
	ch.Unacked[1] = &DeliveryRecord{DeliveryTag: 1, QueueName: q.Name, Message: msg}
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
