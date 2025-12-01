package management

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type connectionFakeBroker struct {
	*fakeBroker
	connections []amqp.ConnectionInfo
	closeError  error
}

func (cfb *connectionFakeBroker) ListConnections() []amqp.ConnectionInfo {
	return cfb.connections
}

func (cfb *connectionFakeBroker) GetConnectionByName(name string) (*amqp.ConnectionInfo, error) {
	for i := range cfb.connections {
		if cfb.connections[i].Client != nil && cfb.connections[i].Client.RemoteAddr == name {
			return &cfb.connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection '%s' not found", name)
}

func (cfb *connectionFakeBroker) CloseConnection(name string, reason string) error {
	if cfb.closeError != nil {
		return cfb.closeError
	}
	// Simulate connection closure by removing it
	for i, conn := range cfb.connections {
		if conn.Client != nil && conn.Client.RemoteAddr == name {
			cfb.connections = append(cfb.connections[:i], cfb.connections[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("connection '%s' not found", name)
}

func setupTestBrokerWithConnections(t *testing.T) *connectionFakeBroker {
	t.Helper()
	base := setupTestBroker(t).(*fakeBroker)

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	ctx3, cancel3 := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel1()
		cancel2()
		cancel3()
	})

	connections := []amqp.ConnectionInfo{
		{
			VHostName: "/",
			Client: &amqp.AmqpClient{
				Conn:       &net.TCPConn{},
				RemoteAddr: "192.168.1.1:50001",
				Config: &amqp.AmqpClientConfig{
					Username: "guest",
					Protocol: "AMQP 0-9-1",
				},
				Ctx:           ctx1,
				LastHeartbeat: time.Now(),
				ConnectedAt:   time.Now(),
			},
			Channels: map[uint16]*amqp.ChannelState{
				1: {},
				2: {},
			},
			ClosingConnection: false,
		},
		{
			VHostName: "/",
			Client: &amqp.AmqpClient{
				Conn:       &net.TCPConn{},
				RemoteAddr: "192.168.1.2:50002",
				Config: &amqp.AmqpClientConfig{
					Username: "admin",
					Protocol: "AMQP 0-9-1",
				},
				Ctx:           ctx2,
				LastHeartbeat: time.Now(),
				ConnectedAt:   time.Now(),
			},
			Channels: map[uint16]*amqp.ChannelState{
				1: {},
			},
			ClosingConnection: false,
		},
		{
			VHostName: "/",
			Client: &amqp.AmqpClient{
				Conn:       &net.TCPConn{},
				RemoteAddr: "192.168.1.3:50003",
				Config: &amqp.AmqpClientConfig{
					Username: "guest",
					Protocol: "AMQP 0-9-1",
				},
				Ctx:           ctx3,
				LastHeartbeat: time.Now(),
				ConnectedAt:   time.Now(),
			},
			Channels:          map[uint16]*amqp.ChannelState{},
			ClosingConnection: true,
		},
	}

	return &connectionFakeBroker{
		fakeBroker:  base,
		connections: connections,
	}
}

func TestListConnections_Success(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	connections, err := service.ListConnections()
	require.NoError(t, err)

	assert.Len(t, connections, 3)

	// Verify first connection (mapped through DTO)
	assert.Equal(t, "192.168.1.1:50001", connections[0].Name)
	assert.Equal(t, "/", connections[0].VHostName)
	assert.Equal(t, "guest", connections[0].Username)
	assert.Equal(t, "AMQP 0-9-1", connections[0].Protocol)
	assert.Equal(t, 2, connections[0].Channels)
}

func TestListConnections_EmptyList(t *testing.T) {
	broker := setupTestBroker(t)
	service := NewService(broker)

	connections, err := service.ListConnections()
	require.NoError(t, err)

	assert.Empty(t, connections)
}

func TestListConnections_NilBroker(t *testing.T) {
	service := &Service{broker: nil}

	_, err := service.ListConnections()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGetConnection_Success(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	conn, err := service.GetConnection("192.168.1.1:50001")
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, "192.168.1.1:50001", conn.Name)
	assert.Equal(t, "/", conn.VHostName)
	assert.Equal(t, "guest", conn.Username)
	assert.Equal(t, 2, conn.Channels)
}

func TestGetConnection_NotFound(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	_, err := service.GetConnection("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetConnection_NilBroker(t *testing.T) {
	service := &Service{broker: nil}

	_, err := service.GetConnection("conn1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestCloseConnection_Success(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	err := service.CloseConnection("192.168.1.1:50001", "test reason")
	require.NoError(t, err)

	// Verify connection was removed
	connections := broker.ListConnections()
	assert.Len(t, connections, 2)

	// Verify conn1 is not in the list
	for _, conn := range connections {
		assert.NotEqual(t, "192.168.1.1:50001", conn.Client.RemoteAddr)
	}
}

func TestCloseConnection_NotFound(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	err := service.CloseConnection("nonexistent", "test reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCloseConnection_NilBroker(t *testing.T) {
	service := &Service{broker: nil}

	err := service.CloseConnection("conn1", "reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestCloseConnection_WithError(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	broker.closeError = fmt.Errorf("close failed")
	service := NewService(broker)

	err := service.CloseConnection("conn1", "reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "close failed")
}

func TestConnectionService_MultipleOperations(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	// List all connections
	connections, err := service.ListConnections()
	require.NoError(t, err)
	initialCount := len(connections)

	// Get specific connection
	conn, err := service.GetConnection("192.168.1.2:50002")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.2:50002", conn.Name)
	assert.Equal(t, "admin", conn.Username)

	// Close connection
	err = service.CloseConnection("192.168.1.2:50002", "cleanup")
	require.NoError(t, err)

	// Verify count decreased
	connections, err = service.ListConnections()
	require.NoError(t, err)
	assert.Equal(t, initialCount-1, len(connections))

	// Verify connection can't be retrieved
	_, err = service.GetConnection("192.168.1.2:50002")
	assert.Error(t, err)
}

func TestConnectionDTOMapping(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	connections, err := service.ListConnections()
	require.NoError(t, err)
	require.NotEmpty(t, connections)

	// Verify DTO mapping includes all fields
	dto := connections[0]
	assert.NotEmpty(t, dto.Name)
	assert.NotEmpty(t, dto.VHostName)
	assert.NotEmpty(t, dto.Username)
	assert.NotEmpty(t, dto.Protocol)
	assert.NotZero(t, dto.Channels)
}

func TestConnectionService_ChannelCounts(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	// conn1 has 2 channels
	conn1, err := service.GetConnection("192.168.1.1:50001")
	require.NoError(t, err)
	assert.Equal(t, 2, conn1.Channels)

	// conn2 has 1 channel
	conn2, err := service.GetConnection("192.168.1.2:50002")
	require.NoError(t, err)
	assert.Equal(t, 1, conn2.Channels)

	// conn3 has 0 channels
	conn3, err := service.GetConnection("192.168.1.3:50003")
	require.NoError(t, err)
	assert.Equal(t, 0, conn3.Channels)
}

func TestConnectionService_ClosingState(t *testing.T) {
	broker := setupTestBrokerWithConnections(t)
	service := NewService(broker)

	// Get conn3 which is closing
	_, err := service.GetConnection("192.168.1.3:50003")
	require.NoError(t, err)

	// Verify ClosingConnection is true through the raw connection
	rawConn, err := broker.GetConnectionByName("192.168.1.3:50003")
	require.NoError(t, err)
	assert.True(t, rawConn.ClosingConnection)
}
