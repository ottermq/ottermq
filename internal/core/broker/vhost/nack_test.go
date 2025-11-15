package vhost

import (
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

func TestHandleBasicNack_Single_RequeueTrue(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	// create queue
	q, err := vh.CreateQueue("q1", nil, conn)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// setup channel delivery state
	key := ConnectionChannelKey{conn, 1}
	ch := &ChannelDeliveryState{Unacked: make(map[uint64]*DeliveryRecord)}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// add one unacked record
	msg := amqp.Message{ID: "m5", Body: []byte("x")}
	ch.mu.Lock()
	ch.Unacked[5] = &DeliveryRecord{
		DeliveryTag: 5,
		ConsumerTag: "ctag",
		QueueName:   "q1",
		Message:     msg,
		Persistent:  false,
	}
	ch.mu.Unlock()

	if err := vh.HandleBasicNack(conn, 1, 5, false, true); err != nil {
		t.Fatalf("HandleBasicNack failed: %v", err)
	}

	// Unacked should be cleared for tag 5
	ch.mu.Lock()
	_, exists := ch.Unacked[5]
	ch.mu.Unlock()
	if exists {
		t.Error("expected delivery tag 5 to be removed from Unacked")
	}

	// Message requeued
	if q.Len() != 1 {
		t.Errorf("expected queue size 1, got %d", q.Len())
	}

	// Marked for redelivery
	if !vh.ShouldRedeliver("m5") {
		t.Error("expected message m5 to be marked for redelivery")
	}
}

func TestHandleBasicNack_Multiple_Boundary_DiscardPersistent(t *testing.T) {
	sp := &dummy.DummyPersistence{
		DeletedMessagesDetailed: []dummy.DeleteRecord{}, // Enable detailed tracking
	}
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     sp,
		EnableDLX:       true, // Enable DLX, but queue has no DLX config, so messages will be discarded
	}
	vh := NewVhost("test-vhost", options)
	var conn net.Conn = nil
	// ensure queue exists (name referenced in records)
	if _, err := vh.CreateQueue("q1", nil, conn); err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	key := ConnectionChannelKey{conn, 1}
	ch := &ChannelDeliveryState{Unacked: make(map[uint64]*DeliveryRecord)}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// tags 1..4, mark 1 and 2 as persistent to check deletion
	msgs := []amqp.Message{
		{ID: "m1"}, {ID: "m2"}, {ID: "m3"}, {ID: "m4"},
	}
	ch.mu.Lock()
	ch.Unacked[1] = &DeliveryRecord{DeliveryTag: 1, ConsumerTag: "c", QueueName: "q1", Message: msgs[0], Persistent: true}
	ch.Unacked[2] = &DeliveryRecord{DeliveryTag: 2, ConsumerTag: "c", QueueName: "q1", Message: msgs[1], Persistent: true}
	ch.Unacked[3] = &DeliveryRecord{DeliveryTag: 3, ConsumerTag: "c", QueueName: "q1", Message: msgs[2], Persistent: false}
	ch.Unacked[4] = &DeliveryRecord{DeliveryTag: 4, ConsumerTag: "c", QueueName: "q1", Message: msgs[3], Persistent: false}
	ch.mu.Unlock()

	// Nack up to tag 2 (<= 2), multiple=true, requeue=false
	if err := vh.HandleBasicNack(conn, 1, 2, true, false); err != nil {
		t.Fatalf("HandleBasicNack failed: %v", err)
	}

	// Expect tags 1,2 removed; 3,4 remain
	ch.mu.Lock()
	_, ex1 := ch.Unacked[1]
	_, ex2 := ch.Unacked[2]
	_, ex3 := ch.Unacked[3]
	_, ex4 := ch.Unacked[4]
	ch.mu.Unlock()

	if ex1 || ex2 {
		t.Error("expected tags 1 and 2 to be removed")
	}
	if !ex3 || !ex4 {
		t.Error("expected tags 3 and 4 to remain")
	}

	// Expect persistence deletions for m1 and m2
	if len(sp.DeletedMessagesDetailed) != 2 {
		t.Fatalf("expected 2 DeleteMessage calls, got %d", len(sp.DeletedMessagesDetailed))
	}
	got := map[string]bool{}
	for _, d := range sp.DeletedMessagesDetailed {
		got[d.MsgID] = true
	}
	if !got["m1"] || !got["m2"] {
		t.Errorf("expected deletes for m1 and m2, got %#v", sp.DeletedMessagesDetailed)
	}
}

func TestHandleBasicNack_NoChannelState(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	err := vh.HandleBasicNack(conn, 1, 1, false, true)
	if err == nil {
		t.Fatal("expected error when channel state missing, got nil")
	}
}

func TestHandleBasicNack_Multiple_AboveBoundaryUnaffected(t *testing.T) {
	var options = VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     nil,
	}
	vh := NewVhost("/", options)
	var conn net.Conn = nil
	if _, err := vh.CreateQueue("q1", nil, conn); err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}
	key := ConnectionChannelKey{conn, 1}
	ch := &ChannelDeliveryState{Unacked: make(map[uint64]*DeliveryRecord)}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// tags 1..4
	ch.mu.Lock()
	for i := uint64(1); i <= 4; i++ {
		ch.Unacked[i] = &DeliveryRecord{DeliveryTag: i, ConsumerTag: "c", QueueName: "q1", Message: amqp.Message{ID: "m"}}
	}
	ch.mu.Unlock()

	if err := vh.HandleBasicNack(conn, 1, 2, true, true); err != nil {
		t.Fatalf("HandleBasicNack failed: %v", err)
	}

	// 1 and 2 removed, 3 and 4 remain
	ch.mu.Lock()
	_, ex1 := ch.Unacked[1]
	_, ex2 := ch.Unacked[2]
	_, ex3 := ch.Unacked[3]
	_, ex4 := ch.Unacked[4]
	ch.mu.Unlock()

	if ex1 || ex2 {
		t.Error("expected tags 1 and 2 to be removed")
	}
	if !ex3 || !ex4 {
		t.Error("expected tags 3 and 4 to remain")
	}
}
