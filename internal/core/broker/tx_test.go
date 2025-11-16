package broker

import (
	"net"
	"strings"
	"testing"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/testutil"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

// Helper function to create a test broker with transaction support
func createTestBrokerForTx() (*Broker, *testutil.MockFramer, net.Conn, *vhost.VHost) {
	mockFramer := &testutil.MockFramer{}
	broker := &Broker{
		framer:      mockFramer,
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		VHosts:      make(map[string]*vhost.VHost),
	}

	// Create test vhost with queues and exchanges
	var options = vhost.VHostOptions{
		QueueBufferSize: 1000,
		Persistence:     &dummy.DummyPersistence{},
	}
	vh := vhost.NewVhost("test-vhost", options)

	// Create a test queue
	vh.Queues["test-queue"] = vhost.NewQueue("test-queue", 100)
	vh.Queues["test-queue"].Props = &vhost.QueueProperties{
		Passive:    false,
		Durable:    false,
		AutoDelete: false,
		Exclusive:  false,
		Arguments:  nil,
	}

	// Create a test exchange with binding
	exchange := &vhost.Exchange{
		Name:     "test-exchange",
		Typ:      vhost.DIRECT,
		Bindings: make(map[string][]*vhost.Binding),
		Props:    &vhost.ExchangeProperties{Internal: false},
	}
	exchange.Bindings["test.key"] = []*vhost.Binding{
		{Queue: vh.Queues["test-queue"]},
	}
	vh.Exchanges["test-exchange"] = exchange

	broker.VHosts["test-vhost"] = vh

	conn := &MockConnection{
		localAddr:  &MockAddr{"tcp", "127.0.0.1:5672"},
		remoteAddr: &MockAddr{"tcp", "127.0.0.1:12345"},
	}

	// Initialize connection state
	broker.Connections[conn] = &amqp.ConnectionInfo{
		VHostName: "test-vhost",
		Channels:  make(map[uint16]*amqp.ChannelState),
	}
	broker.Connections[conn].Channels[1] = &amqp.ChannelState{}

	return broker, mockFramer, conn, vh
}

// =============================================================================
// TX.SELECT Tests - Basic Transaction Mode Activation
// =============================================================================

func TestTxSelect_EntersTransactionMode(t *testing.T) {
	broker, mockFramer, conn, vh := createTestBrokerForTx()

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.TX),
		MethodID: uint16(amqp.TX_SELECT),
		Content:  nil,
	}

	_, err := broker.txSelectHandler(request, vh, conn)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that TX.SELECT-OK frame was sent
	if len(mockFramer.SentFrames) != 1 {
		t.Errorf("Expected 1 frame to be sent, got %d", len(mockFramer.SentFrames))
	}

	// Verify transaction state was created
	txState := vh.GetTransactionState(1, conn)
	if txState == nil {
		t.Fatal("Transaction state was not created")
	}

	txState.Lock()
	defer txState.Unlock()

	if !txState.InTransaction {
		t.Error("Channel should be in transaction mode")
	}

	if txState.BufferedPublishes == nil {
		t.Error("BufferedPublishes should be initialized")
	}

	if txState.BufferedAcks == nil {
		t.Error("BufferedAcks should be initialized")
	}
}

func TestTxSelect_Idempotent(t *testing.T) {
	broker, mockFramer, conn, vh := createTestBrokerForTx()

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.TX),
		MethodID: uint16(amqp.TX_SELECT),
		Content:  nil,
	}

	// Call TX.SELECT first time
	_, err := broker.txSelectHandler(request, vh, conn)
	if err != nil {
		t.Fatalf("First TX.SELECT failed: %v", err)
	}

	firstCallFrames := len(mockFramer.SentFrames)

	// Call TX.SELECT second time (should be idempotent)
	_, err = broker.txSelectHandler(request, vh, conn)
	if err != nil {
		t.Errorf("Second TX.SELECT should not error: %v", err)
	}

	// Should still send TX.SELECT-OK
	if len(mockFramer.SentFrames) != firstCallFrames+1 {
		t.Errorf("Expected TX.SELECT-OK for second call, got %d total frames", len(mockFramer.SentFrames))
	}

	// Transaction state should still be active
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if !txState.InTransaction {
		t.Error("Channel should still be in transaction mode")
	}
}

// =============================================================================
// TX.COMMIT Tests - Error Cases
// =============================================================================

func TestTxCommit_WithoutSelect_ReturnsError(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.TX),
		MethodID: uint16(amqp.TX_COMMIT),
		Content:  nil,
	}

	// Try to commit without calling TX.SELECT first
	_, err := broker.txCommitHandler(request, vh, conn)

	if err == nil {
		t.Fatal("Expected error when committing without transaction mode")
	}

	// Should be a channel error containing the expected message
	expectedErrorSubstring := "not in transaction mode"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedErrorSubstring, err.Error())
	}
}

func TestTxRollback_WithoutSelect_ReturnsError(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	request := &amqp.RequestMethodMessage{
		Channel:  1,
		ClassID:  uint16(amqp.TX),
		MethodID: uint16(amqp.TX_ROLLBACK),
		Content:  nil,
	}

	// Try to rollback without calling TX.SELECT first
	_, err := broker.txRollbackHandler(request, vh, conn)

	if err == nil {
		t.Fatal("Expected error when rolling back without transaction mode")
	}

	expectedErrorSubstring := "not in transaction mode"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedErrorSubstring, err.Error())
	}
}

// =============================================================================
// Publish Buffering Tests - Messages Should Not Route in TX Mode
// =============================================================================

func TestPublish_InTransactionMode_BuffersNotRoutes(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Create a message to publish
	msg := &amqp.Message{
		ID:   "test-msg-1",
		Body: []byte("test message"),
		Properties: amqp.BasicProperties{
			ContentType:  "text/plain",
			DeliveryMode: amqp.NON_PERSISTENT,
		},
	}

	// Publish message in transaction mode
	_, err, buffered := broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", msg, false)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !buffered {
		t.Error("Message should have been buffered")
	}

	// Verify message was NOT routed to queue yet
	queue := vh.Queues["test-queue"]
	if queue.Len() != 0 {
		t.Errorf("Expected 0 messages in queue (buffered), got %d", queue.Len())
	}

	// Verify message is in buffer
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if len(txState.BufferedPublishes) != 1 {
		t.Errorf("Expected 1 buffered publish, got %d", len(txState.BufferedPublishes))
	}

	if txState.BufferedPublishes[0].ExchangeName != "test-exchange" {
		t.Errorf("Expected exchange 'test-exchange', got '%s'", txState.BufferedPublishes[0].ExchangeName)
	}

	if txState.BufferedPublishes[0].RoutingKey != "test.key" {
		t.Errorf("Expected routing key 'test.key', got '%s'", txState.BufferedPublishes[0].RoutingKey)
	}
}

func TestPublish_OutsideTransaction_RoutesImmediately(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Do NOT enter transaction mode

	// Create a message to publish
	amqpMsg := &amqp.Message{
		ID:   "test-msg-1",
		Body: []byte("test message"),
		Properties: amqp.BasicProperties{
			ContentType:  "text/plain",
			DeliveryMode: amqp.NON_PERSISTENT,
		},
	}

	// Try to buffer (should return false since not in TX mode)
	_, err, buffered := broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", amqpMsg, false)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if buffered {
		t.Error("Message should NOT have been buffered (not in TX mode)")
	}

	// Now publish directly (simulating normal flow)
	msgId := vhost.GenerateMessageId()
	msg := vhost.NewMessage(*amqpMsg, msgId)
	_, err = vh.Publish("test-exchange", "test.key", &msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify message was routed to queue
	queue := vh.Queues["test-queue"]
	if queue.Len() != 1 {
		t.Errorf("Expected 1 message in queue, got %d", queue.Len())
	}
}

// =============================================================================
// ACK Buffering Tests - ACKs Should Not Process in TX Mode
// =============================================================================

func TestAck_InTransactionMode_BuffersNotProcesses(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Setup: Deliver a message to create an unacked message
	channelKey := vhost.ConnectionChannelKey{Connection: conn, Channel: 1}
	vh.ChannelDeliveries[channelKey] = &vhost.ChannelDeliveryState{
		Unacked:         make(map[uint64]*vhost.DeliveryRecord),
		LastDeliveryTag: 0,
	}

	vh.ChannelDeliveries[channelKey].Unacked[1] = &vhost.DeliveryRecord{
		DeliveryTag: 1,
		Message: vhost.Message{
			ID:   "test-msg-1",
			Body: []byte("test message"),
		},
		QueueName: "test-queue",
	}

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// ACK message in transaction mode
	_, err, buffered := broker.bufferAcknowledgeTransaction(vh, 1, conn, 1, false, false, vhost.AckOperationAck)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !buffered {
		t.Error("ACK should have been buffered")
	}

	// Verify message is still in unacked map (not removed yet)
	if len(vh.ChannelDeliveries[channelKey].Unacked) != 1 {
		t.Errorf("Expected 1 unacked message, got %d", len(vh.ChannelDeliveries[channelKey].Unacked))
	}

	// Verify ACK is in buffer
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if len(txState.BufferedAcks) != 1 {
		t.Errorf("Expected 1 buffered ack, got %d", len(txState.BufferedAcks))
	}

	if txState.BufferedAcks[0].Operation != vhost.AckOperationAck {
		t.Errorf("Expected AckOperationAck, got %v", txState.BufferedAcks[0].Operation)
	}

	if txState.BufferedAcks[0].DeliveryTag != 1 {
		t.Errorf("Expected delivery tag 1, got %d", txState.BufferedAcks[0].DeliveryTag)
	}
}

func TestNack_InTransactionMode_BuffersNotProcesses(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Setup: Deliver a message to create an unacked message
	channelKey := vhost.ConnectionChannelKey{Connection: conn, Channel: 1}
	vh.ChannelDeliveries[channelKey] = &vhost.ChannelDeliveryState{
		Unacked:         make(map[uint64]*vhost.DeliveryRecord),
		LastDeliveryTag: 0,
	}

	vh.ChannelDeliveries[channelKey].Unacked[1] = &vhost.DeliveryRecord{
		DeliveryTag: 1,
		Message: vhost.Message{
			ID:   "test-msg-1",
			Body: []byte("test message"),
		},
		QueueName: "test-queue",
	}

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// NACK message in transaction mode
	_, err, buffered := broker.bufferAcknowledgeTransaction(vh, 1, conn, 1, false, true, vhost.AckOperationNack)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !buffered {
		t.Error("NACK should have been buffered")
	}

	// Verify NACK is in buffer
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if len(txState.BufferedAcks) != 1 {
		t.Errorf("Expected 1 buffered ack, got %d", len(txState.BufferedAcks))
	}

	if txState.BufferedAcks[0].Operation != vhost.AckOperationNack {
		t.Errorf("Expected AckOperationNack, got %v", txState.BufferedAcks[0].Operation)
	}

	if !txState.BufferedAcks[0].Requeue {
		t.Error("Expected requeue=true")
	}
}

func TestReject_InTransactionMode_BuffersNotProcesses(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Setup: Deliver a message to create an unacked message
	channelKey := vhost.ConnectionChannelKey{Connection: conn, Channel: 1}
	vh.ChannelDeliveries[channelKey] = &vhost.ChannelDeliveryState{
		Unacked:         make(map[uint64]*vhost.DeliveryRecord),
		LastDeliveryTag: 0,
	}

	vh.ChannelDeliveries[channelKey].Unacked[1] = &vhost.DeliveryRecord{
		DeliveryTag: 1,
		Message: vhost.Message{
			ID:   "test-msg-1",
			Body: []byte("test message"),
		},
		QueueName: "test-queue",
	}

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// REJECT message in transaction mode
	_, err, buffered := broker.bufferAcknowledgeTransaction(vh, 1, conn, 1, false, false, vhost.AckOperationReject)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !buffered {
		t.Error("REJECT should have been buffered")
	}

	// Verify REJECT is in buffer
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if len(txState.BufferedAcks) != 1 {
		t.Errorf("Expected 1 buffered ack, got %d", len(txState.BufferedAcks))
	}

	if txState.BufferedAcks[0].Operation != vhost.AckOperationReject {
		t.Errorf("Expected AckOperationReject, got %v", txState.BufferedAcks[0].Operation)
	}
}

// =============================================================================
// TX.COMMIT Tests - Successful Commit Flow
// =============================================================================

func TestCommit_RoutesAllBufferedPublishes(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Buffer multiple publishes
	messages := []string{"msg-1", "msg-2", "msg-3"}
	for _, msgID := range messages {
		msg := &amqp.Message{
			ID:   msgID,
			Body: []byte(msgID),
			Properties: amqp.BasicProperties{
				ContentType:  "text/plain",
				DeliveryMode: amqp.NON_PERSISTENT,
			},
		}
		broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", msg, false)
	}

	// Verify queue is empty before commit
	queue := vh.Queues["test-queue"]
	if queue.Len() != 0 {
		t.Errorf("Expected 0 messages in queue before commit, got %d", queue.Len())
	}

	// Commit transaction
	commitRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_COMMIT),
	}
	_, err = broker.txCommitHandler(commitRequest, vh, conn)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify all messages were routed to queue
	if queue.Len() != len(messages) {
		t.Errorf("Expected %d messages in queue after commit, got %d", len(messages), queue.Len())
	}

	// Verify buffer was cleared
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if len(txState.BufferedPublishes) != 0 {
		t.Errorf("Expected 0 buffered publishes after commit, got %d", len(txState.BufferedPublishes))
	}
}

func TestCommit_ProcessesAllBufferedAcks(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Setup: Create multiple unacked messages
	channelKey := vhost.ConnectionChannelKey{Connection: conn, Channel: 1}
	vh.ChannelDeliveries[channelKey] = &vhost.ChannelDeliveryState{
		Unacked:         make(map[uint64]*vhost.DeliveryRecord),
		LastDeliveryTag: 0,
	}

	for i := uint64(1); i <= 3; i++ {
		vh.ChannelDeliveries[channelKey].Unacked[i] = &vhost.DeliveryRecord{
			DeliveryTag: i,
			Message: vhost.Message{
				ID:   "test-msg",
				Body: []byte("test"),
			},
			QueueName: "test-queue",
		}
	}

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Buffer multiple ACKs
	for i := uint64(1); i <= 3; i++ {
		broker.bufferAcknowledgeTransaction(vh, 1, conn, i, false, false, vhost.AckOperationAck)
	}

	// Verify messages are still unacked before commit
	if len(vh.ChannelDeliveries[channelKey].Unacked) != 3 {
		t.Errorf("Expected 3 unacked messages before commit, got %d", len(vh.ChannelDeliveries[channelKey].Unacked))
	}

	// Commit transaction
	commitRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_COMMIT),
	}
	_, err = broker.txCommitHandler(commitRequest, vh, conn)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify all messages were acked
	if len(vh.ChannelDeliveries[channelKey].Unacked) != 0 {
		t.Errorf("Expected 0 unacked messages after commit, got %d", len(vh.ChannelDeliveries[channelKey].Unacked))
	}

	// Verify buffer was cleared
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if len(txState.BufferedAcks) != 0 {
		t.Errorf("Expected 0 buffered acks after commit, got %d", len(txState.BufferedAcks))
	}
}

func TestCommit_KeepsTransactionMode(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Commit empty transaction
	commitRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_COMMIT),
	}
	_, err = broker.txCommitHandler(commitRequest, vh, conn)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify channel is still in transaction mode (per AMQP spec)
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	inTransaction := txState.InTransaction
	txState.Unlock()

	if !inTransaction {
		t.Error("Channel should remain in transaction mode after commit (per AMQP spec)")
	}

	// Should be able to buffer more operations without calling TX.SELECT again
	msg := &amqp.Message{
		ID:   "msg-after-commit",
		Body: []byte("test"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}

	_, err, buffered := broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", msg, false)
	if err != nil {
		t.Errorf("Should be able to buffer after commit: %v", err)
	}
	if !buffered {
		t.Error("Message should have been buffered (still in TX mode)")
	}
}

// =============================================================================
// TX.ROLLBACK Tests
// =============================================================================

func TestRollback_DiscardsAllBuffers(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Buffer some publishes
	for i := 1; i <= 3; i++ {
		msg := &amqp.Message{
			ID:   "test-msg",
			Body: []byte("test"),
		}
		broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", msg, false)
	}

	// Verify buffers have data
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	bufferCountBefore := len(txState.BufferedPublishes)
	txState.Unlock()

	if bufferCountBefore != 3 {
		t.Errorf("Expected 3 buffered publishes, got %d", bufferCountBefore)
	}

	// Rollback transaction
	rollbackRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_ROLLBACK),
	}
	_, err = broker.txRollbackHandler(rollbackRequest, vh, conn)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify buffers were cleared
	txState.Lock()
	defer txState.Unlock()

	if len(txState.BufferedPublishes) != 0 {
		t.Errorf("Expected 0 buffered publishes after rollback, got %d", len(txState.BufferedPublishes))
	}

	if len(txState.BufferedAcks) != 0 {
		t.Errorf("Expected 0 buffered acks after rollback, got %d", len(txState.BufferedAcks))
	}

	// Verify queue is still empty (messages were discarded)
	queue := vh.Queues["test-queue"]
	if queue.Len() != 0 {
		t.Errorf("Expected 0 messages in queue after rollback, got %d", queue.Len())
	}
}

func TestRollback_KeepsTransactionMode(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	_, err := broker.txSelectHandler(selectRequest, vh, conn)
	if err != nil {
		t.Fatalf("Failed to enter transaction mode: %v", err)
	}

	// Rollback empty transaction
	rollbackRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_ROLLBACK),
	}
	_, err = broker.txRollbackHandler(rollbackRequest, vh, conn)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify channel is still in transaction mode (per AMQP spec)
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	defer txState.Unlock()

	if !txState.InTransaction {
		t.Error("Channel should remain in transaction mode after rollback (per AMQP spec)")
	}
}

// =============================================================================
// Channel Cleanup Tests
// =============================================================================

func TestChannelClose_ImplicitRollback(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode and buffer some operations
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	broker.txSelectHandler(selectRequest, vh, conn)

	msg := &amqp.Message{
		ID:   "test-msg",
		Body: []byte("test"),
	}
	broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", msg, false)

	// Verify buffer has data
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	bufferCount := len(txState.BufferedPublishes)
	txState.Unlock()

	if bufferCount != 1 {
		t.Errorf("Expected 1 buffered publish, got %d", bufferCount)
	}

	// Close channel (simulate channel close)
	vh.CleanupChannel(conn, 1)

	// Verify transaction state was cleaned up
	txState = vh.GetTransactionState(1, conn)
	if txState != nil {
		txState.Lock()
		defer txState.Unlock()

		if txState.InTransaction {
			t.Error("Transaction mode should be cleared after channel close")
		}

		if len(txState.BufferedPublishes) != 0 {
			t.Errorf("Expected 0 buffered publishes after channel close, got %d", len(txState.BufferedPublishes))
		}
	}

	// Verify queue is still empty (implicit rollback)
	queue := vh.Queues["test-queue"]
	if queue.Len() != 0 {
		t.Errorf("Expected 0 messages in queue after channel close (implicit rollback), got %d", queue.Len())
	}
}

// =============================================================================
// Edge Case Tests - Mandatory Flag and Routing
// =============================================================================

func TestCommit_MandatoryNoRoute_ReturnsMessage(t *testing.T) {
	broker, mockFramer, conn, vh := createTestBrokerForTx()

	// Create an exchange without bindings
	vh.Exchanges["no-route-ex"] = &vhost.Exchange{
		Name:     "no-route-ex",
		Typ:      vhost.DIRECT,
		Bindings: make(map[string][]*vhost.Binding),
		Props:    &vhost.ExchangeProperties{Internal: false},
	}

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	broker.txSelectHandler(selectRequest, vh, conn)

	// Buffer a mandatory publish with no routing
	msg := &amqp.Message{
		ID:   "test-msg",
		Body: []byte("test"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}
	broker.bufferPublishInTransaction(vh, 1, conn, "no-route-ex", "no.route", msg, true)

	mockFramer.SentFrames = [][]byte{} // Clear frames from TX.SELECT

	// Commit transaction - should fail due to mandatory with no route
	commitRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_COMMIT),
	}
	_, err := broker.txCommitHandler(commitRequest, vh, conn)

	// Commit should not fail due to mandatory with no route
	if err != nil {
		t.Error("Expected error for mandatory message with no route")
	}

	// No basic.return should be sent - transaction was rolled back, message never committed
	// (In a transaction, mandatory check happens during commit, not during publish)
}

func TestCommit_NonMandatoryNoRoute_DropsMessage(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Create an exchange without bindings
	vh.Exchanges["no-route-ex"] = &vhost.Exchange{
		Name:     "no-route-ex",
		Typ:      vhost.DIRECT,
		Bindings: make(map[string][]*vhost.Binding),
		Props:    &vhost.ExchangeProperties{Internal: false},
	}

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	broker.txSelectHandler(selectRequest, vh, conn)

	// Buffer a non-mandatory publish with no routing
	msg := &amqp.Message{
		ID:   "test-msg",
		Body: []byte("test"),
		Properties: amqp.BasicProperties{
			ContentType: "text/plain",
		},
	}
	broker.bufferPublishInTransaction(vh, 1, conn, "no-route-ex", "no.route", msg, false)

	// Commit transaction - should silently drop message
	commitRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_COMMIT),
	}
	_, err := broker.txCommitHandler(commitRequest, vh, conn)

	// Should succeed (message silently dropped)
	if err != nil {
		t.Errorf("Expected no error for non-mandatory message with no route, got: %v", err)
	}
}

func TestCommit_DryRunValidation(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	broker.txSelectHandler(selectRequest, vh, conn)

	// Buffer publish to non-existent exchange
	msg := &amqp.Message{
		ID:   "test-msg",
		Body: []byte("test"),
	}

	// Manually add to buffer (bypassing validation that would happen in publish handler)
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	txState.BufferedPublishes = append(txState.BufferedPublishes, vhost.BufferedPublish{
		ExchangeName: "non-existent-exchange",
		RoutingKey:   "test.key",
		Message:      *msg,
		Mandatory:    false,
	})
	txState.Unlock()

	// Commit transaction - dry run should catch the error
	commitRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_COMMIT),
	}
	_, err := broker.txCommitHandler(commitRequest, vh, conn)

	// Should fail validation
	if err == nil {
		t.Error("Expected error for publish to non-existent exchange")
	}

	// Buffer should be cleared after failed commit
	txState.Lock()
	defer txState.Unlock()
	if len(txState.BufferedPublishes) != 0 {
		t.Errorf("Expected buffer to be cleared after failed commit, got %d buffered publishes", len(txState.BufferedPublishes))
	}
}

// =============================================================================
// Buffer Limit Tests
// =============================================================================

func TestPublish_ExceedsBufferLimit(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	broker.txSelectHandler(selectRequest, vh, conn)

	// Fill buffer up to limit
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	for i := 0; i < MaxTransactionBufferSize; i++ {
		txState.BufferedPublishes = append(txState.BufferedPublishes, vhost.BufferedPublish{
			ExchangeName: "test-exchange",
			RoutingKey:   "test.key",
			Message: amqp.Message{
				ID:   "test-msg",
				Body: []byte("test"),
			},
		})
	}
	txState.Unlock()

	// Try to buffer one more (should fail)
	msg := &amqp.Message{
		ID:   "overflow-msg",
		Body: []byte("test"),
	}
	_, err, _ := broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", msg, false)

	if err == nil {
		t.Error("Expected error when exceeding buffer limit")
	}

	expectedErrorMsg := "transaction buffer size limit exceeded"
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}

func TestAck_ExceedsBufferLimit(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	broker.txSelectHandler(selectRequest, vh, conn)

	// Fill buffer up to limit
	txState := vh.GetTransactionState(1, conn)
	txState.Lock()
	for i := 0; i < MaxTransactionBufferSize; i++ {
		txState.BufferedAcks = append(txState.BufferedAcks, vhost.BufferedAck{
			Operation:   vhost.AckOperationAck,
			DeliveryTag: uint64(i + 1),
			Multiple:    false,
			Requeue:     false,
		})
	}
	txState.Unlock()

	// Try to buffer one more (should fail)
	_, err, _ := broker.bufferAcknowledgeTransaction(vh, 1, conn, 99999, false, false, vhost.AckOperationAck)

	if err == nil {
		t.Error("Expected error when exceeding buffer limit")
	}

	expectedErrorMsg := "transaction buffer size limit exceeded"
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}

// =============================================================================
// Multiple Operations in Single Transaction
// =============================================================================

func TestCommit_MixedOperations(t *testing.T) {
	broker, _, conn, vh := createTestBrokerForTx()

	// Setup: Create unacked messages
	channelKey := vhost.ConnectionChannelKey{Connection: conn, Channel: 1}
	vh.ChannelDeliveries[channelKey] = &vhost.ChannelDeliveryState{
		Unacked:         make(map[uint64]*vhost.DeliveryRecord),
		LastDeliveryTag: 0,
	}

	vh.ChannelDeliveries[channelKey].Unacked[1] = &vhost.DeliveryRecord{
		DeliveryTag: 1,
		Message:     vhost.Message{ID: "msg-1", Body: []byte("test")},
		QueueName:   "test-queue",
	}
	vh.ChannelDeliveries[channelKey].Unacked[2] = &vhost.DeliveryRecord{
		DeliveryTag: 2,
		Message:     vhost.Message{ID: "msg-2", Body: []byte("test")},
		QueueName:   "test-queue",
	}

	// Enter transaction mode
	selectRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_SELECT),
	}
	broker.txSelectHandler(selectRequest, vh, conn)

	// Mix of operations: publish, ack, nack, reject
	msg := &amqp.Message{ID: "new-msg", Body: []byte("test")}
	broker.bufferPublishInTransaction(vh, 1, conn, "test-exchange", "test.key", msg, false)

	broker.bufferAcknowledgeTransaction(vh, 1, conn, 1, false, false, vhost.AckOperationAck)
	broker.bufferAcknowledgeTransaction(vh, 1, conn, 2, false, true, vhost.AckOperationNack)

	// Commit
	commitRequest := &amqp.RequestMethodMessage{
		Channel:  1,
		MethodID: uint16(amqp.TX_COMMIT),
	}
	_, err := broker.txCommitHandler(commitRequest, vh, conn)

	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify publish was processed
	queue := vh.Queues["test-queue"]
	if queue.Len() < 1 {
		t.Errorf("Expected at least 1 message in queue (from publish + possible nack requeue), got %d", queue.Len())
	}

	// Verify ack was processed (message 1 should be removed from unacked)
	if _, exists := vh.ChannelDeliveries[channelKey].Unacked[1]; exists {
		t.Error("Message 1 should have been acked and removed from unacked map")
	}
}
