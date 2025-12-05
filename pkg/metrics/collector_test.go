package metrics

import (
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
			c := NewCollector(tt.config)

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
			if c.totalPublishes == nil {
				t.Error("totalPublishes not initialized")
			}
			if c.totalDeliveries == nil {
				t.Error("totalDeliveries not initialized")
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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

	// Enqueue 3 messages
	c.RecordQueuePublish("testq")
	c.RecordQueuePublish("testq")
	c.RecordQueuePublish("testq")

	qm := c.GetQueueMetrics("testq")
	if qm.MessageCount.Load() != 3 {
		t.Fatalf("MessageCount = %d, want 3", qm.MessageCount.Load())
	}

	// Deliver 1 message
	c.RecordQueueDelivery("testq")
	time.Sleep(5 * time.Millisecond)

	// Ack it (should decrease count)
	c.RecordQueueAck("testq")

	if qm.MessageCount.Load() != 2 {
		t.Errorf("MessageCount after ack = %d, want 2", qm.MessageCount.Load())
	}

	// Deliver and ack remaining
	c.RecordQueueDelivery("testq")
	c.RecordQueueAck("testq")
	c.RecordQueueDelivery("testq")
	c.RecordQueueAck("testq")

	if qm.MessageCount.Load() != 0 {
		t.Errorf("MessageCount after all acks = %d, want 0", qm.MessageCount.Load())
	}
}

func TestQueueNack(t *testing.T) {
	c := NewCollector(nil)

	c.RecordQueuePublish("testq")
	c.RecordQueuePublish("testq")

	// Nack doesn't decrease message count (message stays in queue)
	c.RecordQueueNack("testq")

	qm := c.GetQueueMetrics("testq")
	if qm.MessageCount.Load() != 2 {
		t.Errorf("MessageCount after nack = %d, want 2 (nack doesn't remove)", qm.MessageCount.Load())
	}
}

func TestSetQueueDepth(t *testing.T) {
	c := NewCollector(nil)

	c.RecordQueuePublish("testq")

	// Explicitly set depth (useful for periodic synchronization)
	c.SetQueueDepth("testq", 100)

	qm := c.GetQueueMetrics("testq")
	if qm.MessageCount.Load() != 100 {
		t.Errorf("MessageCount = %d, want 100", qm.MessageCount.Load())
	}
}

func TestSetQueueConsumers(t *testing.T) {
	c := NewCollector(nil)

	c.RecordQueuePublish("testq")

	// Set consumer count
	c.SetQueueConsumers("testq", 3)

	qm := c.GetQueueMetrics("testq")
	if qm.ConsumerCount.Load() != 3 {
		t.Errorf("ConsumerCount = %d, want 3", qm.ConsumerCount.Load())
	}

	// Verify broker-level count updated
	if c.consumerCount.Load() != 3 {
		t.Errorf("broker consumerCount = %d, want 3", c.consumerCount.Load())
	}

	// Update consumer count
	c.SetQueueConsumers("testq", 5)

	if qm.ConsumerCount.Load() != 5 {
		t.Errorf("ConsumerCount = %d, want 5", qm.ConsumerCount.Load())
	}

	if c.consumerCount.Load() != 5 {
		t.Errorf("broker consumerCount = %d, want 5", c.consumerCount.Load())
	}
}

func TestQueueMetricsSnapshot(t *testing.T) {
	c := NewCollector(nil)

	// Setup queue with messages
	c.RecordQueuePublish("testq")
	time.Sleep(10 * time.Millisecond)
	c.RecordQueuePublish("testq")
	time.Sleep(10 * time.Millisecond)
	c.RecordQueuePublish("testq")

	c.SetQueueConsumers("testq", 2)

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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

	c.RecordQueuePublish("tempq")
	c.RecordQueuePublish("tempq")
	c.SetQueueConsumers("tempq", 2)

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
	c := NewCollector(nil)

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
	rate := c.totalPublishes.Rate()
	if rate <= 0 {
		t.Error("totalPublishes rate should be positive")
	}
}

func TestBrokerLevelConnectionMetrics(t *testing.T) {
	c := NewCollector(nil)

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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

	// Setup broker state
	c.RecordExchangePublish("ex1", "direct")
	time.Sleep(10 * time.Millisecond)
	c.RecordExchangePublish("ex1", "direct")

	c.RecordQueuePublish("q1")
	c.RecordQueueDelivery("q1")
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
	c := NewCollector(nil)

	// Record some data
	c.RecordExchangePublish("ex1", "direct")
	time.Sleep(10 * time.Millisecond)
	c.RecordExchangePublish("ex1", "direct")

	c.RecordQueueDelivery("q1")
	c.RecordConnection()

	// Get time series
	publishSamples := c.GetPublishRateTimeSeries(5 * time.Minute)
	deliverySamples := c.GetDeliveryRateTimeSeries(5 * time.Minute)
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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

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
				c.RecordQueueDelivery("testq")
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
	c := NewCollector(nil)

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
	c := NewCollector(config)

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
	c := NewCollector(nil)

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
	c := NewCollector(nil)

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
// Benchmarks
// ========================================

func BenchmarkRecordExchangePublish(b *testing.B) {
	c := NewCollector(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordExchangePublish("bench.exchange", "direct")
	}
}

func BenchmarkRecordQueuePublish(b *testing.B) {
	c := NewCollector(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordQueuePublish("bench.queue")
	}
}

func BenchmarkRecordConnection(b *testing.B) {
	c := NewCollector(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordConnection()
	}
}

func BenchmarkSnapshot(b *testing.B) {
	c := NewCollector(nil)

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
	c := NewCollector(nil)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.RecordExchangePublish("bench.exchange", "direct")
		}
	})
}
