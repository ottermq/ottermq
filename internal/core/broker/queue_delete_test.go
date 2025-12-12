package broker

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/pkg/metrics"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
	"github.com/google/uuid"
)

// Reuse mockConn from queue_test.go in this package
func newTestConsumerConnID() vhost.ConnectionID {
	return vhost.ConnectionID(fmt.Sprintf("%s:%s", "test-host", uuid.New().String()[:8]))
}

func TestQueueDeleteHandler_IfUnusedBlocksWhenConsumersExist(t *testing.T) {
	mockCollector := metrics.NewCollector(&metrics.Config{
		Enabled: false,
	}, context.Background())
	b := &Broker{
		framer:      &amqp.DefaultFramer{},
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		collector:   mockCollector,
	}
	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)
	vh.SetMetricsCollector(mockCollector)
	connOwner := &mockConn{}
	connOwnerID := newTestConsumerConnID()

	// Create queue
	q, err := vh.CreateQueue("qdel.ifunused", &vhost.QueueProperties{Durable: false}, connOwnerID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}
	_ = q

	// Register a consumer to make queue 'in use'
	consumer := vhost.NewConsumer(connOwnerID, 1, "qdel.ifunused", "ctag-1", &vhost.ConsumerProperties{})
	if _, err := vh.RegisterConsumer(consumer); err != nil {
		t.Fatalf("RegisterConsumer failed: %v", err)
	}

	// Prepare broker connection/channel state for channel.close
	b.mu.Lock()
	b.Connections[connOwner] = &amqp.ConnectionInfo{Channels: map[uint16]*amqp.ChannelState{1: {}}}
	b.mu.Unlock()

	// Initialize and register connection ID in broker's map
	b.connectionsMu.Lock()
	if b.connToID == nil {
		b.connToID = make(map[net.Conn]vhost.ConnectionID)
	}
	b.connToID[connOwner] = connOwnerID
	b.connectionsMu.Unlock()

	// Build request with IfUnused=true
	req := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_DELETE),
		Content: &amqp.QueueDeleteMessage{
			QueueName: "qdel.ifunused",
			IfUnused:  true,
			IfEmpty:   false,
			NoWait:    false,
		},
	}

	_, err = b.queueDeleteHandler(req, vh, connOwner)
	if err != nil {
		t.Fatalf("queueDeleteHandler returned error: %v", err)
	}

	// Channel should be marked closing
	b.mu.Lock()
	if !b.Connections[connOwner].Channels[1].ClosingChannel {
		b.mu.Unlock()
		t.Fatalf("expected channel to be marked closing")
	}
	b.mu.Unlock()

	// Queue should still exist
	if _, exists := vh.Queues["qdel.ifunused"]; !exists {
		t.Fatalf("queue should not be deleted when IfUnused=true and consumers exist")
	}
}

func TestQueueDeleteHandler_IfEmptyBlocksWhenMessagesExist(t *testing.T) {
	b := &Broker{framer: &amqp.DefaultFramer{}, Connections: make(map[net.Conn]*amqp.ConnectionInfo)}
	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)
	conn := &mockConn{}
	connID := newTestConsumerConnID()

	// Create queue and seed messages
	q, err := vh.CreateQueue("qdel.ifempty", &vhost.QueueProperties{Durable: false}, connID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}
	q.Push(vhost.Message{ID: "1", Body: []byte("A")})

	b.mu.Lock()
	b.Connections[conn] = &amqp.ConnectionInfo{Channels: map[uint16]*amqp.ChannelState{1: {}}}
	b.mu.Unlock()

	// Initialize and register connection ID
	b.connectionsMu.Lock()
	if b.connToID == nil {
		b.connToID = make(map[net.Conn]vhost.ConnectionID)
	}
	b.connToID[conn] = connID
	b.connectionsMu.Unlock()

	req := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_DELETE),
		Content: &amqp.QueueDeleteMessage{
			QueueName: "qdel.ifempty",
			IfUnused:  false,
			IfEmpty:   true,
			NoWait:    false,
		},
	}

	_, err = b.queueDeleteHandler(req, vh, conn)
	if err != nil {
		t.Fatalf("queueDeleteHandler returned error: %v", err)
	}

	// Channel should be closing
	b.mu.Lock()
	if !b.Connections[conn].Channels[1].ClosingChannel {
		b.mu.Unlock()
		t.Fatalf("expected channel to be marked closing")
	}
	b.mu.Unlock()

	// Queue should still exist
	if _, exists := vh.Queues["qdel.ifempty"]; !exists {
		t.Fatalf("queue should not be deleted when IfEmpty=true and messages exist")
	}
}

func TestQueueDeleteHandler_SuccessDeletesQueueAndSendsOk(t *testing.T) {
	mockCollector := metrics.NewCollector(&metrics.Config{
		Enabled: false,
	}, context.Background())
	b := &Broker{
		framer:      &amqp.DefaultFramer{},
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		collector:   mockCollector,
	}
	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)
	vh.SetMetricsCollector(mockCollector)
	connID := newTestConsumerConnID()

	// Create queue and seed messages
	q, err := vh.CreateQueue("qdel.ok", &vhost.QueueProperties{Durable: false}, connID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}
	q.Push(vhost.Message{ID: "1"})
	q.Push(vhost.Message{ID: "2"})

	conn := &mockConn{}

	// Initialize and register connection ID
	b.connectionsMu.Lock()
	if b.connToID == nil {
		b.connToID = make(map[net.Conn]vhost.ConnectionID)
	}
	b.connToID[conn] = connID
	b.connectionsMu.Unlock()

	req := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_DELETE),
		Content: &amqp.QueueDeleteMessage{
			QueueName: "qdel.ok",
			IfUnused:  false,
			IfEmpty:   false,
			NoWait:    false,
		},
	}

	_, err = b.queueDeleteHandler(req, vh, conn)
	if err != nil {
		t.Fatalf("queueDeleteHandler returned error: %v", err)
	}

	// Ok frame should have been written
	if len(conn.written) == 0 {
		t.Fatalf("expected delete-ok frame to be sent")
	}

	// Queue should be removed
	if _, exists := vh.Queues["qdel.ok"]; exists {
		t.Fatalf("expected queue to be deleted")
	}
}

func TestQueueDeleteHandler_ExclusiveOwnerMismatch_AccessRefused(t *testing.T) {
	b := &Broker{framer: &amqp.DefaultFramer{}, Connections: make(map[net.Conn]*amqp.ConnectionInfo)}
	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)
	connOwnerID := newTestConsumerConnID()

	// Owner connection declares exclusive queue
	// owner := &mockConn{}
	q, err := vh.CreateQueue("qdel.excl", &vhost.QueueProperties{Durable: false, Exclusive: true}, connOwnerID)
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}
	if q.OwnerConn != connOwnerID {
		t.Fatalf("expected owner to be set on exclusive queue")
	}

	// Another connection attempts delete
	foreign := &mockConn{}
	foreignID := newTestConsumerConnID()
	b.mu.Lock()
	b.Connections[foreign] = &amqp.ConnectionInfo{Channels: map[uint16]*amqp.ChannelState{1: {}}}
	b.mu.Unlock()

	// Initialize and register foreign connection ID
	b.connectionsMu.Lock()
	if b.connToID == nil {
		b.connToID = make(map[net.Conn]vhost.ConnectionID)
	}
	b.connToID[foreign] = foreignID
	b.connectionsMu.Unlock()

	req := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_DELETE),
		Content: &amqp.QueueDeleteMessage{
			QueueName: "qdel.excl",
			IfUnused:  false,
			IfEmpty:   false,
			NoWait:    false,
		},
	}

	_, err = b.queueDeleteHandler(req, vh, foreign)
	if err != nil {
		t.Fatalf("queueDeleteHandler returned error: %v", err)
	}

	// Channel should be closing due to ACCESS_REFUSED
	b.mu.Lock()
	if !b.Connections[foreign].Channels[1].ClosingChannel {
		b.mu.Unlock()
		t.Fatalf("expected channel to be marked closing")
	}
	b.mu.Unlock()

	// Queue must still exist
	if _, exists := vh.Queues["qdel.excl"]; !exists {
		t.Fatalf("exclusive queue should not be deleted by non-owner")
	}
}
