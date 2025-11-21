package broker

import (
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

// Mock connection for testing
type mockConn struct {
	net.Conn
	written    []byte
	remoteAddr net.Addr
}

func (m *mockConn) Write(b []byte) (int, error) {
	m.written = append(m.written, b...)
	return len(b), nil
}

func (m *mockConn) RemoteAddr() net.Addr {
	if m.remoteAddr != nil {
		return m.remoteAddr
	}
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5672}
}

type mockAddr struct {
	addr string
}

func (m *mockAddr) Network() string { return "tcp" }
func (m *mockAddr) String() string  { return m.addr }

func TestQueuePurgeHandler_Success(t *testing.T) {
	b := &Broker{
		framer:   &amqp.DefaultFramer{},
		connToID: make(map[net.Conn]vhost.ConnectionID),
	}

	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)
	connID := newTestConsumerConnID()
	// Create queue and seed messages
	vh.CreateQueue("purge-q", &vhost.QueueProperties{Durable: false}, connID)
	vh.Queues["purge-q"].Push(vhost.Message{ID: "1", Body: []byte("a")})
	vh.Queues["purge-q"].Push(vhost.Message{ID: "2", Body: []byte("b")})

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_PURGE),
		Content: &amqp.QueuePurgeMessage{
			QueueName: "purge-q",
			NoWait:    false,
		},
	}

	conn := &mockConn{}
	// Register connection ID
	b.connToID[conn] = connID

	_, err := b.queuePurgeHandler(request, vh, conn)
	if err != nil {
		t.Fatalf("queuePurgeHandler failed: %v", err)
	}

	if len(conn.written) == 0 {
		t.Error("Expected purge-ok frame to be sent")
	}

	if got := vh.Queues["purge-q"].Len(); got != 0 {
		t.Errorf("Expected queue to be empty after purge, got %d", got)
	}
}

func TestQueuePurgeHandler_QueueNotFound_SendsChannelClose(t *testing.T) {
	b := &Broker{
		framer:      &amqp.DefaultFramer{},
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		connToID:    make(map[net.Conn]vhost.ConnectionID),
	}

	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)

	conn := &mockConn{}
	connID := newTestConsumerConnID()

	// Register connection and channel so sendChannelErrorResponse can mark closing
	b.mu.Lock()
	b.Connections[conn] = &amqp.ConnectionInfo{Channels: make(map[uint16]*amqp.ChannelState)}
	b.Connections[conn].Channels[1] = &amqp.ChannelState{}
	b.mu.Unlock()

	// Register connection ID
	b.connToID[conn] = connID

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_PURGE),
		Content:  &amqp.QueuePurgeMessage{QueueName: "ghost", NoWait: false},
	}

	_, err := b.queuePurgeHandler(request, vh, conn)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(conn.written) == 0 {
		t.Error("Expected channel.close frame to be sent")
	}

	b.mu.Lock()
	if !b.Connections[conn].Channels[1].ClosingChannel {
		t.Error("Expected channel to be marked as closing")
	}
	b.mu.Unlock()
}

func TestQueuePurgeHandler_InvalidContentType(t *testing.T) {
	b := &Broker{
		framer: &amqp.DefaultFramer{},
	}

	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_PURGE),
		Content:  &amqp.QueueBindMessage{}, // wrong type
	}

	conn := &mockConn{}

	_, err := b.queuePurgeHandler(request, vh, conn)
	if err == nil {
		t.Fatal("Expected error for invalid content type")
	}

	expected := "invalid content type for QueuePurgeMessage"
	if err.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, err.Error())
	}
}

func TestQueueUnbindHandler_Success(t *testing.T) {
	b := &Broker{
		framer:   &amqp.DefaultFramer{},
		connToID: make(map[net.Conn]vhost.ConnectionID),
	}

	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)
	connID := newTestConsumerConnID()

	// Create exchange and queue
	vh.CreateExchange("test-exchange", vhost.DIRECT, &vhost.ExchangeProperties{Durable: false})
	vh.CreateQueue("test-queue", &vhost.QueueProperties{Durable: false}, connID)
	vh.BindQueue("test-exchange", "test-queue", "test.key", nil, connID)

	// Create unbind request
	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_UNBIND),
		Content: &amqp.QueueUnbindMessage{
			Queue:      "test-queue",
			Exchange:   "test-exchange",
			RoutingKey: "test.key",
			Arguments:  nil,
		},
	}

	conn := &mockConn{}
	// Register connection ID
	b.connToID[conn] = connID

	// Execute handler
	_, err := b.queueUnbindHandler(request, vh, conn)

	if err != nil {
		t.Fatalf("queueUnbindHandler failed: %v", err)
	}

	// Verify unbind-ok was sent
	if len(conn.written) == 0 {
		t.Error("Expected unbind-ok frame to be sent")
	}

	// Verify binding was removed
	exchange := vh.Exchanges["test-exchange"]
	if len(exchange.Bindings["test.key"]) != 0 {
		t.Error("Expected binding to be removed")
	}
}

func TestQueueUnbindHandler_ExchangeNotFound(t *testing.T) {
	b := &Broker{
		framer:      &amqp.DefaultFramer{},
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		connToID:    make(map[net.Conn]vhost.ConnectionID),
	}

	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)
	vh.SetFramer(b.framer)

	connID := newTestConsumerConnID()

	// Create only queue, no exchange
	vh.CreateQueue("test-queue", &vhost.QueueProperties{Durable: false}, connID)
	conn := &mockConn{}

	// Register connection and channel in broker
	b.mu.Lock()
	b.Connections[conn] = &amqp.ConnectionInfo{
		Channels: make(map[uint16]*amqp.ChannelState),
	}
	b.Connections[conn].Channels[1] = &amqp.ChannelState{
		ClosingChannel: false,
	}
	b.mu.Unlock()

	// Register connection ID
	b.connToID[conn] = connID

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_UNBIND),
		Content: &amqp.QueueUnbindMessage{
			Queue:      "test-queue",
			Exchange:   "ghost-exchange",
			RoutingKey: "test.key",
			Arguments:  nil,
		},
	}

	// Execute handler - should send channel.close
	_, err := b.queueUnbindHandler(request, vh, conn)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify channel.close was sent (not unbind-ok)
	if len(conn.written) == 0 {
		t.Error("Expected channel.close frame to be sent")
	}

	// Verify it's a channel.close frame by checking frame type
	if len(conn.written) >= 8 {
		frameType := conn.written[0]
		if frameType != 1 { // METHOD frame
			t.Errorf("Expected METHOD frame (type 1), got type %d", frameType)
		}
	}

	// Verify channel is marked as closing
	b.mu.Lock()
	if !b.Connections[conn].Channels[1].ClosingChannel {
		t.Error("Expected channel to be marked as closing")
	}
	b.mu.Unlock()
}

func TestQueueUnbindHandler_InvalidContentType(t *testing.T) {
	b := &Broker{
		framer: &amqp.DefaultFramer{},
	}

	var options = vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("/", options)

	// Create request with wrong content type
	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.QUEUE),
		MethodID: uint16(amqp.QUEUE_UNBIND),
		Content:  &amqp.QueueBindMessage{}, // Wrong type!
	}

	conn := &mockConn{}

	// Execute handler
	_, err := b.queueUnbindHandler(request, vh, conn)

	if err == nil {
		t.Fatal("Expected error for invalid content type")
	}

	expectedMsg := "invalid content type for QueueUnbindMessage"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}
