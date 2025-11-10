package broker

import (
	"net"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/testutil"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

// Test Suite: Broker-Level Channel Flow Control
//
// Tests the broker's handling of AMQP channel.flow and channel.flow-ok methods.
//
// Coverage:
// - handleChannelFlow: Processes client-initiated flow control requests
// - handleChannelFlowOk: Acknowledges server-initiated flow (currently no-op)
// - Integration with vhost flow state management
// - Error handling for invalid content and non-existent channels
//
// Note: Complex publish scenarios are covered by e2e tests to avoid
// dependency on exchange/queue/binding setup complexity.

// TestHandleChannelFlow_ClientRequest tests handling of client-initiated channel.flow
func TestHandleChannelFlow_ClientRequest(t *testing.T) {
	mockFramer := &testutil.MockFramer{}
	broker := &Broker{
		framer:      mockFramer,
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		VHosts:      make(map[string]*vhost.VHost),
	}

	vh := vhost.NewVhost("test-vhost", 1000, &dummy.DummyPersistence{})
	broker.VHosts["test-vhost"] = vh

	conn := &MockConnection{
		localAddr:  &MockAddr{network: "tcp", address: "127.0.0.1:5672"},
		remoteAddr: &MockAddr{network: "tcp", address: "127.0.0.1:12345"},
	}

	// Initialize connection
	broker.Connections[conn] = &amqp.ConnectionInfo{
		VHostName: "test-vhost",
		Channels:  make(map[uint16]*amqp.ChannelState),
	}
	broker.Connections[conn].Channels[1] = &amqp.ChannelState{}

	// Test 1: Client requests flow pause (active=false)
	request := &amqp.RequestMethodMessage{
		Channel: 1,
		Content: &amqp.ChannelFlowContent{
			Active: false,
		},
	}

	_, err := broker.handleChannelFlow(request, vh, conn)
	if err != nil {
		t.Fatalf("handleChannelFlow failed: %v", err)
	}

	// Verify flow state was updated
	flowState := vh.GetChannelFlowState(conn, 1)
	if flowState.FlowActive {
		t.Error("Expected FlowActive to be false after client pause")
	}
	if flowState.FlowInitiatedByBroker {
		t.Error("Expected FlowInitiatedByBroker to be false for client request")
	}

	// Verify flow-ok was sent
	if len(mockFramer.SentFrames) != 1 {
		t.Errorf("Expected 1 frame to be sent, got %d", len(mockFramer.SentFrames))
	}

	// Test 2: Client requests flow resume (active=true)
	mockFramer.SentFrames = [][]byte{} // Reset
	request.Content = &amqp.ChannelFlowContent{
		Active: true,
	}

	_, err = broker.handleChannelFlow(request, vh, conn)
	if err != nil {
		t.Fatalf("handleChannelFlow failed: %v", err)
	}

	flowState = vh.GetChannelFlowState(conn, 1)
	if !flowState.FlowActive {
		t.Error("Expected FlowActive to be true after client resume")
	}

	if len(mockFramer.SentFrames) != 1 {
		t.Errorf("Expected 1 frame to be sent, got %d", len(mockFramer.SentFrames))
	}
}

// TestHandleChannelFlow_InvalidContent tests error handling for invalid content
func TestHandleChannelFlow_InvalidContent(t *testing.T) {
	mockFramer := &testutil.MockFramer{}
	broker := &Broker{
		framer:      mockFramer,
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		VHosts:      make(map[string]*vhost.VHost),
	}

	vh := vhost.NewVhost("test-vhost", 1000, &dummy.DummyPersistence{})
	broker.VHosts["test-vhost"] = vh

	conn := &MockConnection{
		localAddr:  &MockAddr{network: "tcp", address: "127.0.0.1:5672"},
		remoteAddr: &MockAddr{network: "tcp", address: "127.0.0.1:12345"},
	}

	broker.Connections[conn] = &amqp.ConnectionInfo{
		VHostName: "test-vhost",
		Channels:  make(map[uint16]*amqp.ChannelState),
	}
	broker.Connections[conn].Channels[1] = &amqp.ChannelState{}

	// Test with wrong content type
	request := &amqp.RequestMethodMessage{
		Channel: 1,
		Content: &amqp.ChannelOpenContent{}, // Wrong type
	}

	_, err := broker.handleChannelFlow(request, vh, conn)
	if err == nil {
		t.Error("Expected error for invalid content type")
	}
}

// TestHandleChannelFlow_NonExistentChannel tests handling flow on non-opened channel
func TestHandleChannelFlow_NonExistentChannel(t *testing.T) {
	mockFramer := &testutil.MockFramer{}
	broker := &Broker{
		framer:      mockFramer,
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		VHosts:      make(map[string]*vhost.VHost),
	}

	vh := vhost.NewVhost("test-vhost", 1000, &dummy.DummyPersistence{})
	broker.VHosts["test-vhost"] = vh

	conn := &MockConnection{
		localAddr:  &MockAddr{network: "tcp", address: "127.0.0.1:5672"},
		remoteAddr: &MockAddr{network: "tcp", address: "127.0.0.1:12345"},
	}

	broker.Connections[conn] = &amqp.ConnectionInfo{
		VHostName: "test-vhost",
		Channels:  make(map[uint16]*amqp.ChannelState),
	}
	// Note: Channel 99 is NOT in Channels map

	request := &amqp.RequestMethodMessage{
		Channel: 99,
		Content: &amqp.ChannelFlowContent{
			Active: false,
		},
	}

	_, err := broker.handleChannelFlow(request, vh, conn)
	// Should return error for non-existent channel
	if err == nil {
		t.Error("Expected error for non-existent channel")
	}
}

// TestBasicPublish_WithClientFlowPaused verifies client-initiated flow doesn't block publishing
func TestBasicPublish_WithClientFlowPaused(t *testing.T) {
	t.Skip("Complex test requiring full exchange/queue setup - covered by e2e tests")
}

// TestBasicPublish_WithServerFlowPaused verifies server-initiated flow closes channel on publish
func TestBasicPublish_WithServerFlowPaused(t *testing.T) {
	t.Skip("Complex test requiring full message assembly - covered by e2e tests")
}

// TestHandleChannelFlowOk tests that flow-ok is accepted without error
func TestHandleChannelFlowOk(t *testing.T) {
	mockFramer := &testutil.MockFramer{}
	broker := &Broker{
		framer:      mockFramer,
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		VHosts:      make(map[string]*vhost.VHost),
	}

	vh := vhost.NewVhost("test-vhost", 1000, &dummy.DummyPersistence{})
	broker.VHosts["test-vhost"] = vh

	conn := &MockConnection{
		localAddr:  &MockAddr{network: "tcp", address: "127.0.0.1:5672"},
		remoteAddr: &MockAddr{network: "tcp", address: "127.0.0.1:12345"},
	}

	broker.Connections[conn] = &amqp.ConnectionInfo{
		VHostName: "test-vhost",
		Channels:  make(map[uint16]*amqp.ChannelState),
	}
	broker.Connections[conn].Channels[1] = &amqp.ChannelState{}

	// Test flow-ok handling
	request := &amqp.RequestMethodMessage{
		Channel: 1,
		Content: &amqp.ChannelFlowOkContent{
			Active: true,
		},
	}

	_, err := broker.handleChannelFlowOk(request, conn)
	if err != nil {
		t.Errorf("handleChannelFlowOk failed: %v", err)
	}

	// Currently flow-ok is just acknowledged, no state change expected
	// (since server doesn't initiate flow in current implementation)
}
