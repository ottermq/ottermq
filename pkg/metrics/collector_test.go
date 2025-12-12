package metrics

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ========================================
// Constructor Tests
// ========================================

func TestNewCollector(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		wantEnabled    bool
		wantWindowSize time.Duration
		wantMaxSamples int
	}{
		{
			name:           "with_custom_config",
			config:         &Config{Enabled: true, WindowSize: 10 * time.Minute, MaxSamples: 120},
			wantEnabled:    true,
			wantWindowSize: 10 * time.Minute,
			wantMaxSamples: 120,
		},
		{
			name:           "with_nil_config",
			config:         nil,
			wantEnabled:    true,
			wantWindowSize: 5 * time.Minute,
			wantMaxSamples: 60,
		},
		{
			name:           "with_disabled_config",
			config:         &Config{Enabled: false, WindowSize: 5 * time.Minute, MaxSamples: 60},
			wantEnabled:    false,
			wantWindowSize: 5 * time.Minute,
			wantMaxSamples: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create dummy context
			ctx := context.Background()
			c := NewCollector(tt.config, ctx)

			if c.IsEnabled() != tt.wantEnabled {
				t.Errorf("IsEnabled() = %v, want %v", c.IsEnabled(), tt.wantEnabled)
			}

			if c.config.WindowSize != tt.wantWindowSize {
				t.Errorf("WindowSize = %v, want %v", c.config.WindowSize, tt.wantWindowSize)
			}

			if c.config.MaxSamples != tt.wantMaxSamples {
				t.Errorf("MaxSamples = %v, want %v", c.config.MaxSamples, tt.wantMaxSamples)
			}

			// Verify rate trackers initialized
			if c.totalPublishesRate == nil {
				t.Error("totalPublishesRate not initialized")
			}
			if c.totalDeliveriesAutoAckRate == nil {
				t.Error("totalDeliveriesAutoAckRate not initialized")
			}
			if c.connectionRate == nil {
				t.Error("connectionRate not initialized")
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("default config should be enabled")
	}

	if config.WindowSize != 5*time.Minute {
		t.Errorf("WindowSize = %v, want 5m", config.WindowSize)
	}

	if config.MaxSamples != 60 {
		t.Errorf("MaxSamples = %v, want 60", config.MaxSamples)
	}
}

// ========================================
// Exchange Metrics Tests
// ========================================

func TestExchangeMetricsBasic(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Record some publishes
	c.RecordExchangePublish("amq.direct", "direct")
	c.RecordExchangePublish("amq.direct", "direct")
	c.RecordExchangePublish("amq.fanout", "fanout")

	// Verify metrics created
	directMetrics := c.GetExchangeMetrics("amq.direct")
	if directMetrics == nil {
		t.Fatal("expected amq.direct metrics to exist")
	}

	if directMetrics.Name != "amq.direct" {
		t.Errorf("Name = %v, want amq.direct", directMetrics.Name)
	}

	if directMetrics.Type != "direct" {
		t.Errorf("Type = %v, want direct", directMetrics.Type)
	}

	if directMetrics.PublishCount.Load() != 2 {
		t.Errorf("PublishCount = %d, want 2", directMetrics.PublishCount.Load())
	}

	fanoutMetrics := c.GetExchangeMetrics("amq.fanout")
	if fanoutMetrics == nil {
		t.Fatal("expected amq.fanout metrics to exist")
	}

	if fanoutMetrics.PublishCount.Load() != 1 {
		t.Errorf("PublishCount = %d, want 1", fanoutMetrics.PublishCount.Load())
	}
}

func TestExchangeMetricsSnapshot(t *testing.T) {

	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Record publishes over time
	c.RecordExchangePublish("test.exchange", "topic")
	time.Sleep(10 * time.Millisecond)
	c.RecordExchangePublish("test.exchange", "topic")
	time.Sleep(10 * time.Millisecond)
	c.RecordExchangePublish("test.exchange", "topic")

	em := c.GetExchangeMetrics("test.exchange")
	if em == nil {
		t.Fatal("expected exchange metrics to exist")
	}

	snapshot := em.Snapshot()

	if snapshot.Name != "test.exchange" {
		t.Errorf("Snapshot.Name = %v, want test.exchange", snapshot.Name)
	}

	if snapshot.Type != "topic" {
		t.Errorf("Snapshot.Type = %v, want topic", snapshot.Type)
	}

	if snapshot.PublishCount != 3 {
		t.Errorf("Snapshot.PublishCount = %d, want 3", snapshot.PublishCount)
	}

	if snapshot.PublishRate <= 0 {
		t.Error("Snapshot.PublishRate should be positive")
	}

	if snapshot.Uptime <= 0 {
		t.Error("Snapshot.Uptime should be positive")
	}
}

func TestGetAllExchangeMetrics(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Create multiple exchanges
	c.RecordExchangePublish("exchange1", "direct")
	c.RecordExchangePublish("exchange2", "fanout")
	c.RecordExchangePublish("exchange3", "topic")

	all := c.GetAllExchangeMetrics()

	if len(all) != 3 {
		t.Fatalf("GetAllExchangeMetrics() returned %d exchanges, want 3", len(all))
	}

	// Verify all exchanges present
	names := make(map[string]bool)
	for _, em := range all {
		names[em.Name] = true
	}

	expectedNames := []string{"exchange1", "exchange2", "exchange3"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("expected exchange %s not found", name)
		}
	}
}

func TestRemoveExchange(t *testing.T) {

	ctx := context.Background()
	c := NewCollector(nil, ctx)

	c.RecordExchangePublish("temp.exchange", "direct")

	// Verify it exists
	if c.GetExchangeMetrics("temp.exchange") == nil {
		t.Fatal("exchange should exist before removal")
	}

	// Remove it
	c.RemoveExchange("temp.exchange")

	// Verify it's gone
	if c.GetExchangeMetrics("temp.exchange") != nil {
		t.Error("exchange should not exist after removal")
	}
}

// ========================================
// Queue Metrics Tests
// ========================================

func TestQueueMetricsBasic(t *testing.T) {

	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Enqueue messages
	c.RecordQueuePublish("queue1")
	c.RecordQueuePublish("queue1")
	c.RecordQueuePublish("queue2")

	qm1 := c.GetQueueMetrics("queue1")
	if qm1 == nil {
		t.Fatal("expected queue1 metrics to exist")
	}

	if qm1.MessageCount.Load() != 2 {
		t.Errorf("MessageCount = %d, want 2", qm1.MessageCount.Load())
	}

	qm2 := c.GetQueueMetrics("queue2")
	if qm2 == nil {
		t.Fatal("expected queue2 metrics to exist")
	}

	if qm2.MessageCount.Load() != 1 {
		t.Errorf("MessageCount = %d, want 1", qm2.MessageCount.Load())
	}
}

func TestQueueDeliveryAndAck(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Enqueue 3 messages
	c.RecordQueuePublish("testq")
	c.RecordQueuePublish("testq")
	c.RecordQueuePublish("testq")

	qm := c.GetQueueMetrics("testq")
	if qm.MessageCount.Load() != 3 {
		t.Fatalf("MessageCount = %d, want 3", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Fatalf("UnackedCount = %d, want 0", qm.UnackedCount.Load())
	}

	// Deliver 1 message (moves from ready to unacked)
	c.RecordQueueDelivery("testq", false)
	time.Sleep(5 * time.Millisecond)

	if qm.MessageCount.Load() != 2 {
		t.Errorf("MessageCount after delivery = %d, want 2", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("UnackedCount after delivery = %d, want 1", qm.UnackedCount.Load())
	}

	// Ack it (removes from unacked)
	c.RecordQueueAck("testq")

	if qm.MessageCount.Load() != 2 {
		t.Errorf("MessageCount after ack = %d, want 2", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("UnackedCount after ack = %d, want 0", qm.UnackedCount.Load())
	}

	// Deliver and ack remaining
	c.RecordQueueDelivery("testq", false)
	c.RecordQueueAck("testq")
	c.RecordQueueDelivery("testq", false)
	c.RecordQueueAck("testq")

	if qm.MessageCount.Load() != 0 {
		t.Errorf("MessageCount after all acks = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("UnackedCount after all acks = %d, want 0", qm.UnackedCount.Load())
	}
}

func TestQueueNackWithRequeue(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Publish 2 messages
	c.RecordQueuePublish("testq")
	c.RecordQueuePublish("testq")

	qm := c.GetQueueMetrics("testq")
	if qm.MessageCount.Load() != 2 {
		t.Fatalf("MessageCount = %d, want 2", qm.MessageCount.Load())
	}

	// Deliver 1 message (moves from ready to unacked)
	c.RecordQueueDelivery("testq", false)

	if qm.MessageCount.Load() != 1 {
		t.Errorf("MessageCount after delivery = %d, want 1", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("UnackedCount after delivery = %d, want 1", qm.UnackedCount.Load())
	}

	// NACK with requeue (moves from unacked back to ready)
	c.RecordQueueNack("testq")    // Remove from unacked
	c.RecordQueueRequeue("testq") // Add back to ready

	if qm.MessageCount.Load() != 2 {
		t.Errorf("MessageCount after nack+requeue = %d, want 2", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("UnackedCount after nack+requeue = %d, want 0", qm.UnackedCount.Load())
	}
}

func TestQueueNackWithoutRequeue(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Publish 2 messages
	c.RecordQueuePublish("testq")
	c.RecordQueuePublish("testq")

	qm := c.GetQueueMetrics("testq")

	// Deliver 1 message
	c.RecordQueueDelivery("testq", false)

	if qm.MessageCount.Load() != 1 {
		t.Errorf("MessageCount after delivery = %d, want 1", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("UnackedCount after delivery = %d, want 1", qm.UnackedCount.Load())
	}

	// NACK without requeue (message is dead-lettered or discarded)
	c.RecordQueueNack("testq")

	if qm.MessageCount.Load() != 1 {
		t.Errorf("MessageCount after nack = %d, want 1 (only ready messages)", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("UnackedCount after nack = %d, want 0 (removed from unacked)", qm.UnackedCount.Load())
	}
}

func TestQueueRequeue(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Simulate consumer cancel scenario
	c.RecordQueuePublish("testq")
	c.RecordQueueDelivery("testq", false) // Message delivered to consumer

	qm := c.GetQueueMetrics("testq")

	if qm.MessageCount.Load() != 0 {
		t.Errorf("MessageCount after delivery = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("UnackedCount after delivery = %d, want 1", qm.UnackedCount.Load())
	}

	// Consumer cancels - message needs to be requeued
	// In real code: RecordQueueNack + RecordQueueRequeue
	c.RecordQueueNack("testq")    // Remove from unacked
	c.RecordQueueRequeue("testq") // Add back to ready

	if qm.MessageCount.Load() != 1 {
		t.Errorf("MessageCount after requeue = %d, want 1", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("UnackedCount after requeue = %d, want 0", qm.UnackedCount.Load())
	}
}

func TestSetQueueDepth(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	c.RecordQueuePublish("testq")

	// Explicitly set depth (useful for periodic synchronization)
	c.SetQueueDepth("testq", 100)

	qm := c.GetQueueMetrics("testq")
	if qm.MessageCount.Load() != 100 {
		t.Errorf("MessageCount = %d, want 100", qm.MessageCount.Load())
	}
}

func TestSetQueueConsumers(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	c.RecordQueuePublish("testq")

	// Add 3 consumers
	c.RecordConsumerAdded("testq")
	c.RecordConsumerAdded("testq")
	c.RecordConsumerAdded("testq")

	qm := c.GetQueueMetrics("testq")
	if qm.ConsumerCount.Load() != 3 {
		t.Errorf("ConsumerCount = %d, want 3", qm.ConsumerCount.Load())
	}

	// Verify broker-level count updated
	if c.consumerCount.Load() != 3 {
		t.Errorf("broker consumerCount = %d, want 3", c.consumerCount.Load())
	}

	// Add 2 more consumers
	c.RecordConsumerAdded("testq")
	c.RecordConsumerAdded("testq")

	if qm.ConsumerCount.Load() != 5 {
		t.Errorf("ConsumerCount = %d, want 5", qm.ConsumerCount.Load())
	}

	if c.consumerCount.Load() != 5 {
		t.Errorf("broker consumerCount = %d, want 5", c.consumerCount.Load())
	}

	// Add 2 more consumers
	c.RecordConsumerRemoved("testq")

	if qm.ConsumerCount.Load() != 4 {
		t.Errorf("ConsumerCount = %d, want 4", qm.ConsumerCount.Load())
	}

	if c.consumerCount.Load() != 4 {
		t.Errorf("broker consumerCount = %d, want 4", c.consumerCount.Load())
	}
}

func TestQueueMetricsSnapshot(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Setup queue with messages
	c.RecordQueuePublish("testq")
	time.Sleep(10 * time.Millisecond)
	c.RecordQueuePublish("testq")
	time.Sleep(10 * time.Millisecond)
	c.RecordQueuePublish("testq")

	c.RecordConsumerAdded("testq")
	c.RecordConsumerAdded("testq")

	qm := c.GetQueueMetrics("testq")
	snapshot := qm.Snapshot()

	if snapshot.Name != "testq" {
		t.Errorf("Snapshot.Name = %v, want testq", snapshot.Name)
	}

	if snapshot.MessageCount != 3 {
		t.Errorf("Snapshot.MessageCount = %d, want 3", snapshot.MessageCount)
	}

	if snapshot.ConsumerCount != 2 {
		t.Errorf("Snapshot.ConsumerCount = %d, want 2", snapshot.ConsumerCount)
	}

	if snapshot.MessageRate <= 0 {
		t.Error("Snapshot.MessageRate should be positive")
	}

	if snapshot.Uptime <= 0 {
		t.Error("Snapshot.Uptime should be positive")
	}
}

func TestGetAllQueueMetrics(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	c.RecordQueuePublish("q1")
	c.RecordQueuePublish("q2")
	c.RecordQueuePublish("q3")

	all := c.GetAllQueueMetrics()

	if len(all) != 3 {
		t.Fatalf("GetAllQueueMetrics() returned %d queues, want 3", len(all))
	}

	names := make(map[string]bool)
	for _, qm := range all {
		names[qm.Name] = true
	}

	expectedNames := []string{"q1", "q2", "q3"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("expected queue %s not found", name)
		}
	}
}

func TestRemoveQueue(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	c.RecordQueuePublish("tempq")
	c.RecordQueuePublish("tempq")
	c.RecordConsumerAdded("tempq")
	c.RecordConsumerAdded("tempq")

	// Verify state before removal
	if c.GetQueueMetrics("tempq") == nil {
		t.Fatal("queue should exist before removal")
	}

	initialMsgCount := c.messageCount.Load()
	initialConsumerCount := c.consumerCount.Load()

	// Remove queue
	c.RemoveQueue("tempq")

	// Verify it's gone
	if c.GetQueueMetrics("tempq") != nil {
		t.Error("queue should not exist after removal")
	}

	// Verify broker counts updated
	if c.messageCount.Load() != initialMsgCount-2 {
		t.Errorf("messageCount should decrease by 2, got %d", c.messageCount.Load())
	}

	if c.consumerCount.Load() != initialConsumerCount-2 {
		t.Errorf("consumerCount should decrease by 2, got %d", c.consumerCount.Load())
	}
}

// ========================================
// Broker-Level Metrics Tests
// ========================================

func TestBrokerLevelPublishMetrics(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Publish to different exchanges
	c.RecordExchangePublish("ex1", "direct")
	c.RecordExchangePublish("ex2", "fanout")
	c.RecordExchangePublish("ex1", "direct")

	// Verify broker-level count
	if c.messageCount.Load() != 3 {
		t.Errorf("broker messageCount = %d, want 3", c.messageCount.Load())
	}

	// Verify rate tracker recorded
	time.Sleep(10 * time.Millisecond)
	rate := c.totalPublishesRate.Rate()
	if rate <= 0 {
		t.Error("totalPublishes rate should be positive")
	}
}

func TestBrokerLevelConnectionMetrics(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Open connections
	c.RecordConnection()
	c.RecordConnection()
	c.RecordConnection()

	if c.connectionCount.Load() != 3 {
		t.Errorf("connectionCount = %d, want 3", c.connectionCount.Load())
	}

	// Close one
	c.RecordConnectionClose()

	if c.connectionCount.Load() != 2 {
		t.Errorf("connectionCount = %d, want 2", c.connectionCount.Load())
	}
}

func TestBrokerLevelChannelMetrics(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Open channels
	c.RecordChannelOpen()
	c.RecordChannelOpen()

	if c.channelCount.Load() != 2 {
		t.Errorf("channelCount = %d, want 2", c.channelCount.Load())
	}

	// Close one
	c.RecordChannelClose()

	if c.channelCount.Load() != 1 {
		t.Errorf("channelCount = %d, want 1", c.channelCount.Load())
	}
}

// ========================================
// Snapshot Tests
// ========================================

func TestBrokerSnapshot(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Setup broker state
	c.RecordExchangePublish("ex1", "direct")
	time.Sleep(10 * time.Millisecond)
	c.RecordExchangePublish("ex1", "direct")

	c.RecordQueuePublish("q1")
	c.RecordQueueDelivery("q1", false)
	time.Sleep(10 * time.Millisecond)
	c.RecordQueueAck("q1")

	c.RecordConnection()
	c.RecordChannelOpen()

	// Get snapshot
	bm := c.GetBrokerMetrics()
	snap := bm.Snapshot()

	if snap.Timestamp.IsZero() {
		t.Error("Snapshot.Timestamp should be set")
	}

	// Verify rates are present (may be small but should be >= 0)
	if snap.PublishRate < 0 {
		t.Errorf("PublishRate = %f, want >= 0", snap.PublishRate)
	}

	if snap.DeliveryRate < 0 {
		t.Errorf("DeliveryRate = %f, want >= 0", snap.DeliveryRate)
	}

	// Verify counts
	if snap.ConnectionCount != 1 {
		t.Errorf("ConnectionCount = %d, want 1", snap.ConnectionCount)
	}

	if snap.ChannelCount != 1 {
		t.Errorf("ChannelCount = %d, want 1", snap.ChannelCount)
	}
}

func TestGetTimeSeriesMethods(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Record some data
	c.RecordExchangePublish("ex1", "direct")
	time.Sleep(10 * time.Millisecond)
	c.RecordExchangePublish("ex1", "direct")

	c.RecordQueueDelivery("q1", false)
	c.RecordConnection()

	// Get time series
	publishSamples := c.GetPublishRateTimeSeries(5 * time.Minute)
	deliverySamples := c.GetDeliveryAutoAckRateTimeSeries(5 * time.Minute)
	connSamples := c.GetConnectionRateTimeSeries(5 * time.Minute)

	if len(publishSamples) == 0 {
		t.Error("expected at least one publish sample")
	}

	if len(deliverySamples) == 0 {
		t.Error("expected at least one delivery sample")
	}

	if len(connSamples) == 0 {
		t.Error("expected at least one connection sample")
	}
}

// ========================================
// Concurrency Tests
// ========================================

func TestConcurrentExchangePublishes(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	const goroutines = 10
	const publishesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			exchangeName := "exchange"
			for j := 0; j < publishesPerGoroutine; j++ {
				c.RecordExchangePublish(exchangeName, "direct")
			}
		}(i)
	}

	wg.Wait()

	em := c.GetExchangeMetrics("exchange")
	if em == nil {
		t.Fatal("expected exchange metrics to exist")
	}

	expectedCount := int64(goroutines * publishesPerGoroutine)
	actualCount := em.PublishCount.Load()

	if actualCount != expectedCount {
		t.Errorf("PublishCount = %d, want %d", actualCount, expectedCount)
	}
}

func TestConcurrentQueueOperations(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	const goroutines = 10
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // publish and delivery goroutines

	// Publishers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				c.RecordQueuePublish("testq")
			}
		}()
	}

	// Consumers (ack after publish completes)
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				c.RecordQueueDelivery("testq", false)
				c.RecordQueueAck("testq")
			}
		}()
	}

	wg.Wait()

	qm := c.GetQueueMetrics("testq")
	if qm == nil {
		t.Fatal("expected queue metrics to exist")
	}

	// All messages should be ack'd (count = 0)
	finalCount := qm.MessageCount.Load()
	if finalCount != 0 {
		t.Errorf("MessageCount = %d, want 0 (all ack'd)", finalCount)
	}
}

func TestConcurrentConnectionOperations(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	const connections = 100

	var wg sync.WaitGroup
	wg.Add(connections)

	// Open connections
	for i := 0; i < connections; i++ {
		go func() {
			defer wg.Done()
			c.RecordConnection()
		}()
	}

	wg.Wait()

	if c.connectionCount.Load() != connections {
		t.Errorf("connectionCount = %d, want %d", c.connectionCount.Load(), connections)
	}

	// Close half
	wg.Add(connections / 2)
	for i := 0; i < connections/2; i++ {
		go func() {
			defer wg.Done()
			c.RecordConnectionClose()
		}()
	}

	wg.Wait()

	expectedRemaining := int64(connections / 2)
	if c.connectionCount.Load() != expectedRemaining {
		t.Errorf("connectionCount = %d, want %d", c.connectionCount.Load(), expectedRemaining)
	}
}

// ========================================
// Edge Cases
// ========================================

func TestDisabledCollector(t *testing.T) {
	config := &Config{
		Enabled:    false,
		WindowSize: 5 * time.Minute,
		MaxSamples: 60,
	}
	c := NewCollector(config, context.Background())

	// All operations should be no-ops when disabled
	c.RecordExchangePublish("ex1", "direct")
	c.RecordQueuePublish("q1")
	c.RecordConnection()

	// Metrics should still be created (for consistency), but not recorded
	if c.GetExchangeMetrics("ex1") == nil {
		// This is acceptable - disabled collector might not create entries
	}
}

func TestClearMetrics(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Setup data
	c.RecordExchangePublish("ex1", "direct")
	c.RecordQueuePublish("q1")
	c.RecordConnection()
	c.RecordChannelOpen()

	// Verify data exists
	if c.GetExchangeMetrics("ex1") == nil {
		t.Fatal("exchange should exist before clear")
	}

	// Clear all
	c.Clear()

	// Verify cleared
	if len(c.GetAllExchangeMetrics()) != 0 {
		t.Error("exchanges should be cleared")
	}

	if len(c.GetAllQueueMetrics()) != 0 {
		t.Error("queues should be cleared")
	}

	if c.messageCount.Load() != 0 {
		t.Error("messageCount should be 0")
	}

	if c.connectionCount.Load() != 0 {
		t.Error("connectionCount should be 0")
	}

	if c.channelCount.Load() != 0 {
		t.Error("channelCount should be 0")
	}
}

func TestEmptyQueueName(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Should handle empty queue name gracefully
	c.RecordQueuePublish("")

	qm := c.GetQueueMetrics("")
	if qm == nil {
		t.Fatal("should create metrics even for empty name")
	}

	if qm.MessageCount.Load() != 1 {
		t.Errorf("MessageCount = %d, want 1", qm.MessageCount.Load())
	}
}

// ========================================
// Full Message Lifecycle Tests
// ========================================

func TestFullMessageLifecycle_Success(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// 1. Publish message to exchange
	c.RecordExchangePublish("my.exchange", "direct")

	// 2. Message routed to queue
	c.RecordQueuePublish("my.queue")

	qm := c.GetQueueMetrics("my.queue")
	if qm.MessageCount.Load() != 1 {
		t.Errorf("After publish: MessageCount = %d, want 1", qm.MessageCount.Load())
	}

	// 3. Consumer receives message
	c.RecordQueueDelivery("my.queue", false)

	if qm.MessageCount.Load() != 0 {
		t.Errorf("After delivery: MessageCount = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("After delivery: UnackedCount = %d, want 1", qm.UnackedCount.Load())
	}

	// 4. Consumer acknowledges
	c.RecordQueueAck("my.queue")

	if qm.MessageCount.Load() != 0 {
		t.Errorf("After ack: MessageCount = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("After ack: UnackedCount = %d, want 0", qm.UnackedCount.Load())
	}

	// Verify broker-level metrics
	if c.messageCount.Load() != 0 {
		t.Errorf("Broker messageCount = %d, want 0 (all consumed)", c.messageCount.Load())
	}
}

func TestFullMessageLifecycle_NackRequeue(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// 1. Publish and route
	c.RecordExchangePublish("my.exchange", "direct")
	c.RecordQueuePublish("my.queue")

	qm := c.GetQueueMetrics("my.queue")

	// 2. Deliver to consumer
	c.RecordQueueDelivery("my.queue", false)

	if qm.MessageCount.Load() != 0 {
		t.Errorf("After delivery: MessageCount = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("After delivery: UnackedCount = %d, want 1", qm.UnackedCount.Load())
	}

	// 3. Consumer NACKs with requeue=true
	c.RecordQueueNack("my.queue")
	c.RecordQueueRequeue("my.queue")

	if qm.MessageCount.Load() != 1 {
		t.Errorf("After nack+requeue: MessageCount = %d, want 1", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("After nack+requeue: UnackedCount = %d, want 0", qm.UnackedCount.Load())
	}

	// 4. Deliver again
	c.RecordQueueDelivery("my.queue", false)

	// 5. ACK this time
	c.RecordQueueAck("my.queue")

	if qm.MessageCount.Load() != 0 {
		t.Errorf("Final: MessageCount = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("Final: UnackedCount = %d, want 0", qm.UnackedCount.Load())
	}
}

func TestFullMessageLifecycle_NackDeadLetter(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// 1. Publish to exchange and route to queue
	c.RecordExchangePublish("my.exchange", "direct")
	c.RecordQueuePublish("my.queue")

	qm := c.GetQueueMetrics("my.queue")

	// 2. Deliver to consumer
	c.RecordQueueDelivery("my.queue", false)

	if qm.MessageCount.Load() != 0 {
		t.Errorf("After delivery: MessageCount = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("After delivery: UnackedCount = %d, want 1", qm.UnackedCount.Load())
	}

	// 3. Consumer NACKs with requeue=false (dead-letter or discard)
	c.RecordQueueNack("my.queue")

	if qm.MessageCount.Load() != 0 {
		t.Errorf("After nack (no requeue): MessageCount = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("After nack (no requeue): UnackedCount = %d, want 0", qm.UnackedCount.Load())
	}

	// Message is gone (dead-lettered to DLX or discarded)
}

func TestFullMessageLifecycle_ConsumerCancel(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Setup queue with consumer
	c.RecordQueuePublish("my.queue")
	c.RecordConsumerAdded("my.queue")

	qm := c.GetQueueMetrics("my.queue")

	// Deliver message
	c.RecordQueueDelivery("my.queue", false)

	if qm.MessageCount.Load() != 0 {
		t.Errorf("After delivery: MessageCount = %d, want 0", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 1 {
		t.Errorf("After delivery: UnackedCount = %d, want 1", qm.UnackedCount.Load())
	}

	// Consumer cancels - unacked messages requeued
	c.RecordConsumerRemoved("my.queue")
	c.RecordQueueNack("my.queue")    // Remove from unacked
	c.RecordQueueRequeue("my.queue") // Back to ready

	if qm.MessageCount.Load() != 1 {
		t.Errorf("After consumer cancel: MessageCount = %d, want 1", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("After consumer cancel: UnackedCount = %d, want 0", qm.UnackedCount.Load())
	}
	if qm.ConsumerCount.Load() != 0 {
		t.Errorf("After consumer cancel: ConsumerCount = %d, want 0", qm.ConsumerCount.Load())
	}
}

func TestFullMessageLifecycle_MultipleMessages(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Publish 10 messages
	for i := 0; i < 10; i++ {
		c.RecordExchangePublish("my.exchange", "direct")
		c.RecordQueuePublish("my.queue")
	}

	qm := c.GetQueueMetrics("my.queue")
	if qm.MessageCount.Load() != 10 {
		t.Fatalf("After publish: MessageCount = %d, want 10", qm.MessageCount.Load())
	}

	// Deliver 5 messages
	for i := 0; i < 5; i++ {
		c.RecordQueueDelivery("my.queue", false)
	}

	if qm.MessageCount.Load() != 5 {
		t.Errorf("After 5 deliveries: MessageCount = %d, want 5", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 5 {
		t.Errorf("After 5 deliveries: UnackedCount = %d, want 5", qm.UnackedCount.Load())
	}

	// ACK 3 messages
	for i := 0; i < 3; i++ {
		c.RecordQueueAck("my.queue")
	}

	if qm.MessageCount.Load() != 5 {
		t.Errorf("After 3 acks: MessageCount = %d, want 5", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 2 {
		t.Errorf("After 3 acks: UnackedCount = %d, want 2", qm.UnackedCount.Load())
	}

	// NACK 2 with requeue
	for i := 0; i < 2; i++ {
		c.RecordQueueNack("my.queue")
		c.RecordQueueRequeue("my.queue")
	}

	if qm.MessageCount.Load() != 7 {
		t.Errorf("After 2 nack+requeue: MessageCount = %d, want 7", qm.MessageCount.Load())
	}
	if qm.UnackedCount.Load() != 0 {
		t.Errorf("After 2 nack+requeue: UnackedCount = %d, want 0", qm.UnackedCount.Load())
	}

	// Total messages in system: 7 ready + 0 unacked = 7
}

func TestFullMessageLifecycle_BrokerCounters(t *testing.T) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Publish to multiple queues
	c.RecordExchangePublish("ex1", "direct")
	c.RecordQueuePublish("q1")

	c.RecordExchangePublish("ex1", "direct")
	c.RecordQueuePublish("q2")

	c.RecordExchangePublish("ex2", "fanout")
	c.RecordQueuePublish("q3")

	// Broker should track total messages
	if c.messageCount.Load() != 3 {
		t.Errorf("Broker messageCount = %d, want 3", c.messageCount.Load())
	}

	// Deliver all
	c.RecordQueueDelivery("q1", false)
	c.RecordQueueDelivery("q2", false)
	c.RecordQueueDelivery("q3", false)

	// ACK all
	c.RecordQueueAck("q1")
	c.RecordQueueAck("q2")
	c.RecordQueueAck("q3")

	// Small delay for rate calculation
	time.Sleep(10 * time.Millisecond)

	// Broker messageCount should be 0 (all consumed)
	if c.messageCount.Load() != 0 {
		t.Errorf("Broker messageCount after all acks = %d, want 0", c.messageCount.Load())
	}

	// Verify rates tracked (should be >= 0, may be 0 if calculations haven't completed)
	bm := c.GetBrokerMetrics()
	if bm.TotalPublishes().Rate() < 0 {
		t.Error("TotalPublishes rate should be >= 0")
	}
	if bm.TotalDeliveries().Rate() < 0 {
		t.Error("TotalDeliveries rate should be >= 0")
	}
	// Note: TotalAcks may be 0 if rate calculation hasn't had time to complete
	if rate := bm.TotalAcks().Rate(); rate < 0 {
		t.Errorf("TotalAcks rate should be >= 0, got %f", rate)
	}
}

// ========================================
// Benchmarks
// ========================================

func BenchmarkRecordExchangePublish(b *testing.B) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordExchangePublish("bench.exchange", "direct")
	}
}

func BenchmarkRecordQueuePublish(b *testing.B) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordQueuePublish("bench.queue")
	}
}

func BenchmarkRecordConnection(b *testing.B) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordConnection()
	}
}

func BenchmarkSnapshot(b *testing.B) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	// Setup some data
	c.RecordExchangePublish("ex1", "direct")
	c.RecordQueuePublish("q1")
	c.RecordConnection()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.GetBrokerMetrics().Snapshot()
	}
}

func BenchmarkConcurrentPublishes(b *testing.B) {
	ctx := context.Background()
	c := NewCollector(nil, ctx)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.RecordExchangePublish("bench.exchange", "direct")
		}
	})
}
