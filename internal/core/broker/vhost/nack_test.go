package vhost

import (
	"testing"

	"github.com/andrelcunha/ottermq/pkg/metrics"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

func TestHandleBasicNack_Single_RequeueTrue(t *testing.T) {
	vh := setupTestVHost()
	connID := newTestConsumerConnID()
	// create queue
	q, err := vh.CreateQueue("q1", nil, connID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	// setup channel delivery state
	key := ConnectionChannelKey{connID, 1}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Register consumer
	c := newTestConsumer(connID, 2, "q-noack", false)

	// add one unacked record
	msg := Message{ID: "m5", Body: []byte("x")}
	ch.mu.Lock()
	record := &DeliveryRecord{
		DeliveryTag: 5,
		ConsumerTag: "ctag",
		QueueName:   "q1",
		Message:     msg,
		Persistent:  false,
	}
	ch.UnackedByTag[5] = record
	if ch.UnackedByConsumer[c.Tag] == nil {
		ch.UnackedByConsumer[c.Tag] = make(map[uint64]*DeliveryRecord)
	}
	ch.UnackedByConsumer[c.Tag][5] = record
	ch.mu.Unlock()

	if err := vh.HandleBasicNack(connID, 1, 5, false, true); err != nil {
		t.Fatalf("HandleBasicNack failed: %v", err)
	}

	// Unacked should be cleared for tag 5
	ch.mu.Lock()
	_, exists := ch.UnackedByTag[5]
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
	vh.SetMetricsCollector(metrics.NewMockCollector(nil))
	connID := newTestConsumerConnID()
	// ensure queue exists (name referenced in records)
	if _, err := vh.CreateQueue("q1", nil, connID); err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}

	key := ConnectionChannelKey{connID, 1}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// Register consumer
	c := newTestConsumer(connID, 2, "q1", false)

	// tags 1..4, mark 1 and 2 as persistent to check deletion
	msgs := []Message{
		{ID: "m1"}, {ID: "m2"}, {ID: "m3"}, {ID: "m4"},
	}
	ch.mu.Lock()
	for i, msg := range msgs {
		deliveryTag := uint64(i + 1)
		record := &DeliveryRecord{
			DeliveryTag: deliveryTag,
			ConsumerTag: c.Tag,
			QueueName:   c.QueueName,
			Message:     msg,
			Persistent:  deliveryTag <= 2, // first two are persistent
		}
		ch.UnackedByTag[deliveryTag] = record
		if ch.UnackedByConsumer[c.Tag] == nil {
			ch.UnackedByConsumer[c.Tag] = make(map[uint64]*DeliveryRecord)
		}
		ch.UnackedByConsumer[c.Tag][deliveryTag] = record
	}
	ch.mu.Unlock()

	// Nack up to tag 2 (<= 2), multiple=true, requeue=false
	if err := vh.HandleBasicNack(connID, 1, 2, true, false); err != nil {
		t.Fatalf("HandleBasicNack failed: %v", err)
	}

	// Expect tags 1,2 removed; 3,4 remain
	ch.mu.Lock()

	for i := 1; i <= 4; i++ {
		deliveryTag := uint64(i)
		_, exists := ch.UnackedByTag[deliveryTag]
		switch {
		case i <= 2:
			if exists {
				t.Errorf("expected tag %d to be removed", i)
			}
		case i >= 3:
			if !exists {
				t.Errorf("expected tag %d to remain", i)
			}
		}
	}
	ch.mu.Unlock()

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
	vh := setupTestVHost()
	connID := newTestConsumerConnID()
	err := vh.HandleBasicNack(connID, 1, 1, false, true)
	if err == nil {
		t.Fatal("expected error when channel state missing, got nil")
	}
}

func TestHandleBasicNack_Multiple_AboveBoundaryUnaffected(t *testing.T) {
	vh := setupTestVHost()
	connID := newTestConsumerConnID()
	if _, err := vh.CreateQueue("q1", nil, connID); err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}
	key := ConnectionChannelKey{connID, 1}
	ch := &ChannelDeliveryState{
		UnackedByTag:      make(map[uint64]*DeliveryRecord),
		UnackedByConsumer: make(map[string]map[uint64]*DeliveryRecord),
	}
	vh.mu.Lock()
	vh.ChannelDeliveries[key] = ch
	vh.mu.Unlock()

	// tags 1..4
	ch.mu.Lock()
	for i := uint64(1); i <= 4; i++ {
		ch.UnackedByTag[i] = &DeliveryRecord{DeliveryTag: i, ConsumerTag: "c", QueueName: "q1", Message: Message{ID: "m"}}
		if ch.UnackedByConsumer["c"] == nil {
			ch.UnackedByConsumer["c"] = make(map[uint64]*DeliveryRecord)
		}
		ch.UnackedByConsumer["c"][i] = ch.UnackedByTag[i]
	}
	ch.mu.Unlock()

	if err := vh.HandleBasicNack(connID, 1, 2, true, true); err != nil {
		t.Fatalf("HandleBasicNack failed: %v", err)
	}

	// 1 and 2 removed, 3 and 4 remain
	ch.mu.Lock()

	for i := 1; i <= 4; i++ {
		deliveryTag := uint64(i)
		_, exists := ch.UnackedByTag[deliveryTag]
		switch {
		case i <= 2:
			if exists {
				t.Errorf("expected tag %d to be removed", i)
			}
		case i >= 3:
			if !exists {
				t.Errorf("expected tag %d to remain", i)
			}
		}
	}
	ch.mu.Unlock()
}
