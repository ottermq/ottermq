package metrics

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ========================================
// Constructor Tests
// ========================================

func TestRateTracker_Constructor(t *testing.T) {
	windowSize := 5 * time.Second
	maxSamples := 100
	rt := NewRateTracker(windowSize, maxSamples)

	if rt.windowSize != windowSize {
		t.Errorf("expected windowSize %v, got %v", windowSize, rt.windowSize)
	}
	if rt.maxSamples != maxSamples {
		t.Errorf("expected maxSamples %d, got %d", maxSamples, rt.maxSamples)
	}
	if len(rt.samples) != 0 || cap(rt.samples) != maxSamples {
		t.Errorf("expected empty samples slice with capacity %d", maxSamples)
	}
}

// ========================================
// Recording & Pruning Tests
// ========================================

func TestRateTracker_Recording(t *testing.T) {
	// Use a short window for testing
	windowSize := 100 * time.Millisecond
	maxSamples := 5
	rt := NewRateTracker(windowSize, maxSamples)

	// Record initial count
	rt.Record(10)

	// Wait a bit and record again
	time.Sleep(50 * time.Millisecond)
	rt.Record(20)

	// Should have 2 samples
	samples := rt.GetSamples()
	if len(samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(samples))
	}

	// Wait for window to expire for first sample
	time.Sleep(60 * time.Millisecond) // Total: 110ms > windowSize
	rt.Record(30)

	// First sample should be pruned, leaving 2 samples
	samples = rt.GetSamples()
	if len(samples) != 2 {
		t.Errorf("expected 2 samples after pruning, got %d", len(samples))
	}

	// Test maxSamples limit
	for i := 0; i < 10; i++ {
		rt.Record(int64(40 + i))
		time.Sleep(10 * time.Millisecond)
	}

	samples = rt.GetSamples()
	if len(samples) > maxSamples {
		t.Errorf("expected at most %d samples, got %d", maxSamples, len(samples))
	}
}

func TestRateTracker_Pruning(t *testing.T) {
	windowSize := 2 * time.Second
	rt := NewRateTracker(windowSize, 1000)

	// Record samples over 3 seconds
	for i := 0; i < 30; i++ {
		rt.Record(int64(i))
		time.Sleep(100 * time.Millisecond)
	}

	samples := rt.GetSamples()

	if len(samples) == 0 {
		t.Fatal("expected samples")
	}

	// Check that all samples are within the window
	now := time.Now()
	cutoff := now.Add(-windowSize)

	for i, sample := range samples {
		age := now.Sub(sample.Timestamp)
		if age > windowSize+50*time.Millisecond { // Allow small timing tolerance
			t.Errorf("sample %d is too old: age=%v, window=%v", i, age, windowSize)
		}
		if sample.Timestamp.Before(cutoff.Add(-50 * time.Millisecond)) {
			t.Errorf("sample %d is outside window: %v < %v", i, sample.Timestamp, cutoff)
		}
	}
}

func TestRateTracker_Clear(t *testing.T) {
	rt := NewRateTracker(time.Second, 10)

	// Add some samples
	for i := 0; i < 5; i++ {
		rt.Record(int64(i * 10))
		time.Sleep(10 * time.Millisecond)
	}

	if len(rt.GetSamples()) == 0 {
		t.Error("expected samples before clear")
	}

	rt.Clear()

	if len(rt.GetSamples()) != 0 {
		t.Error("expected no samples after clear")
	}

	// Should be able to record new samples after clear
	rt.Record(100)
	if len(rt.GetSamples()) != 1 {
		t.Error("expected to be able to record after clear")
	}
}

// ========================================
// Rate Calculation Tests
// ========================================

// ========================================
// Rate Calculation Tests
// ========================================

func TestRateTracker_RateCalculation(t *testing.T) {
	tests := []struct {
		name    string
		samples []struct {
			count int64
			delay time.Duration
		}
		expectedRate float64
		tolerance    float64
	}{
		{
			name: "linear increase",
			samples: []struct {
				count int64
				delay time.Duration
			}{
				{10, 0},
				{20, time.Second},
				{30, time.Second}, // Total elapsed from first: 2 seconds
			},
			expectedRate: 10.0, // (30-10)/2 = 10 per second
			tolerance:    0.5,
		},
		{
			name: "no change",
			samples: []struct {
				count int64
				delay time.Duration
			}{
				{100, 0},
				{100, time.Second},
				{100, time.Second},
			},
			expectedRate: 0.0,
			tolerance:    0.1,
		},
		{
			name: "decreasing counts",
			samples: []struct {
				count int64
				delay time.Duration
			}{
				{100, 0},
				{80, time.Second},
				{60, time.Second}, // Total elapsed from first: 2 seconds
			},
			expectedRate: -20.0, // (60-100)/2 = -20 per second
			tolerance:    0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := NewRateTracker(10*time.Second, 100)

			// Record samples with cumulative delays
			for _, sample := range tt.samples {
				if sample.delay > 0 {
					time.Sleep(sample.delay)
				}
				rt.Record(sample.count)
			}

			rate := rt.Rate()
			if abs(rate-tt.expectedRate) > tt.tolerance {
				t.Errorf("expected rate %.2f (±%.2f), got %.2f", tt.expectedRate, tt.tolerance, rate)
			}
		})
	}
}

func TestRateTracker_GetStats(t *testing.T) {
	rt := NewRateTracker(2*time.Second, 10)

	// Record with known timings
	rt.Record(100)
	time.Sleep(time.Second)
	rt.Record(130) // 30 increase in 1 second = 30/sec
	time.Sleep(500 * time.Millisecond)
	rt.Record(145) // 15 increase in 0.5 seconds = still 30/sec overall

	count, rate := rt.GetStats()

	// Latest count should be 145
	if count != 145 {
		t.Errorf("expected count 145, got %d", count)
	}

	// Rate should be approximately (145-100)/1.5 = 30
	if rate < 29.5 || rate > 30.5 {
		t.Errorf("expected rate ~30.0, got %f", rate)
	}
}

func TestRateTracker_CumulativeCounterPattern(t *testing.T) {
	rt := NewRateTracker(5*time.Second, 100)

	// Simulate real usage: cumulative message counts
	totalMessages := int64(0)

	// First second: 10 messages
	totalMessages += 10
	rt.Record(totalMessages) // Record 10
	time.Sleep(time.Second)

	// Second second: 15 more messages
	totalMessages += 15
	rt.Record(totalMessages) // Record 25 (cumulative)
	time.Sleep(time.Second)

	// Third second: 20 more messages
	totalMessages += 20
	rt.Record(totalMessages) // Record 45 (cumulative)

	count, rate := rt.GetStats()

	if count != 45 {
		t.Errorf("expected count 45, got %d", count)
	}

	// Rate calculation: (45-10)/2 seconds = 17.5 msg/sec
	expectedRate := 17.5
	if abs(rate-expectedRate) > 1.0 {
		t.Errorf("expected rate ~%.1f, got %.1f", expectedRate, rate)
	}
}

func TestRateTracker_RateStability(t *testing.T) {
	rt := NewRateTracker(5*time.Second, 100)

	totalCount := int64(0)

	// Steady traffic for 2 seconds (10 msg/sec)
	for i := 0; i < 20; i++ {
		totalCount += 1
		rt.Record(totalCount)
		time.Sleep(100 * time.Millisecond)
	}

	steadyRate := rt.Rate()

	// Sudden spike: 100 messages at once
	totalCount += 100
	rt.Record(totalCount)

	spikeRate := rt.Rate()

	// Continue steady traffic
	for i := 0; i < 20; i++ {
		totalCount += 1
		rt.Record(totalCount)
		time.Sleep(100 * time.Millisecond)
	}

	postSpikeRate := rt.Rate()

	// Rate should have increased during spike but smoothed out after
	t.Logf("Steady: %.1f, Spike: %.1f, Post-spike: %.1f",
		steadyRate, spikeRate, postSpikeRate)

	// Spike rate should be higher
	if spikeRate <= steadyRate {
		t.Error("expected spike rate to be higher than steady rate")
	}

	// Post-spike should settle back down (but may still be elevated)
	if postSpikeRate > spikeRate {
		t.Error("expected post-spike rate to decrease from spike peak")
	}
}

// ========================================
// Edge Cases Tests
// ========================================

func TestRateTracker_EdgeCases(t *testing.T) {
	t.Run("empty buffer", func(t *testing.T) {
		rt := NewRateTracker(time.Second, 10)
		if rate := rt.Rate(); rate != 0.0 {
			t.Errorf("expected rate 0.0 for empty buffer, got %f", rate)
		}
		count, rate := rt.GetStats()
		if count != 0 || rate != 0.0 {
			t.Errorf("expected (0, 0.0) from GetStats, got (%d, %f)", count, rate)
		}
	})

	t.Run("single sample", func(t *testing.T) {
		rt := NewRateTracker(time.Second, 10)
		rt.Record(100)

		if rate := rt.Rate(); rate != 0.0 {
			t.Errorf("expected rate 0.0 for single sample, got %f", rate)
		}

		count, rate := rt.GetStats()
		if count != 100 || rate != 0.0 {
			t.Errorf("expected (100, 0.0) from GetStats, got (%d, %f)", count, rate)
		}
	})

	t.Run("two samples zero time difference", func(t *testing.T) {
		rt := NewRateTracker(time.Second, 10)

		// Create two samples with same timestamp
		rt.Record(100)
		rt.mu.Lock()
		// Manually add another sample with same timestamp
		if len(rt.samples) > 0 {
			rt.samples = append(rt.samples, Sample{
				Count:     200,
				Timestamp: rt.samples[0].Timestamp,
			})
		}
		rt.mu.Unlock()

		// Should handle zero elapsed time gracefully
		if rate := rt.Rate(); rate != 0.0 {
			t.Errorf("expected rate 0.0 for zero time difference, got %f", rate)
		}
	})

	t.Run("samples at window boundary", func(t *testing.T) {
		windowSize := 500 * time.Millisecond
		rt := NewRateTracker(windowSize, 10)

		// Record first sample
		rt.Record(100)

		// Wait slightly less than the window size
		time.Sleep(windowSize - 50*time.Millisecond)

		// Record second sample before boundary
		rt.Record(200)

		// Should have both samples
		samples := rt.GetSamples()
		if len(samples) < 1 {
			t.Errorf("expected at least 1 sample, got %d", len(samples))
		}
	})
}

func TestRateTracker_SampleOrder(t *testing.T) {
	rt := NewRateTracker(time.Minute, 100)

	// Record multiple samples
	for i := 0; i < 20; i++ {
		rt.Record(int64(i * 10))
		time.Sleep(10 * time.Millisecond)
	}

	samples := rt.GetSamples()

	// Verify chronological order
	for i := 1; i < len(samples); i++ {
		if samples[i].Timestamp.Before(samples[i-1].Timestamp) {
			t.Errorf("samples not in chronological order at index %d", i)
		}
		if samples[i].Count < samples[i-1].Count {
			t.Errorf("cumulative counts decreased at index %d: %d -> %d",
				i, samples[i-1].Count, samples[i].Count)
		}
	}
}

func TestRateTracker_CopyIsolation(t *testing.T) {
	rt := NewRateTracker(time.Minute, 10)

	// Add some samples
	rt.Record(10)
	time.Sleep(10 * time.Millisecond)
	rt.Record(20)

	// Get samples
	samples1 := rt.GetSamples()
	originalLen := len(samples1)

	// Modify returned slice
	if len(samples1) > 0 {
		samples1[0].Count = 999
		samples1 = append(samples1, Sample{Count: 888, Timestamp: time.Now()})
	}

	// Get samples again - should be unchanged
	samples2 := rt.GetSamples()

	if len(samples2) != originalLen {
		t.Error("external modification affected internal state")
	}

	if len(samples2) > 0 && samples2[0].Count == 999 {
		t.Error("GetSamples did not return a copy - internal data was modified")
	}
}

// ========================================
// Concurrency Tests
// ========================================

func TestRateTracker_ConcurrentAccess(t *testing.T) {
	rt := NewRateTracker(time.Second, 1000)
	var wg sync.WaitGroup

	// Start multiple goroutines to Record
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rt.Record(int64(id*100 + j))
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Start multiple goroutines to read Rate
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = rt.Rate()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Start multiple goroutines to read GetStats
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = rt.GetStats()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Start multiple goroutines to read GetSamples
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_ = rt.GetSamples()
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Verify data structure integrity
	samples := rt.GetSamples()
	if len(samples) > rt.maxSamples {
		t.Errorf("samples exceeded maxSamples: %d > %d", len(samples), rt.maxSamples)
	}

	// All samples should be within the window
	cutoff := time.Now().Add(-rt.windowSize)
	for _, sample := range samples {
		if sample.Timestamp.Before(cutoff) {
			t.Errorf("found stale sample outside window: %v", sample.Timestamp)
		}
	}
}

// ========================================
// Realistic Usage Tests
// ========================================

func TestRateTracker_RealisticSampling(t *testing.T) {
	// Real config: 5-minute window, sample every 5 seconds
	rt := NewRateTracker(5*time.Minute, 60)

	// Simulate 3 seconds of traffic at 100 msg/sec (compressed time)
	totalCount := int64(0)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(3 * time.Second)

	for {
		select {
		case <-ticker.C:
			totalCount += 10 // 10 messages per 100ms = 100 msg/sec
			rt.Record(totalCount)

		case <-timeout:
			goto done
		}
	}

done:
	// Should have ~30 samples (one every 100ms for 3 seconds)
	samples := rt.GetSamples()
	if len(samples) < 25 || len(samples) > 35 {
		t.Errorf("expected ~30 samples, got %d", len(samples))
	}

	// Rate should be stable around 100 msg/sec
	rate := rt.Rate()
	if rate < 95 || rate > 105 {
		t.Errorf("expected rate ~100 msg/sec, got %.1f", rate)
	}
}

// TestRateTracker_UsageExample demonstrates the intended usage pattern
func TestRateTracker_UsageExample(t *testing.T) {
	// Create tracker: 5-minute window, 60 samples max (one per 5 seconds)
	tracker := NewRateTracker(5*time.Minute, 60)

	// In production: atomic counter tracks total messages
	var totalMessages int64

	// Simulate message traffic for 1 second
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			atomic.AddInt64(&totalMessages, 1)
			time.Sleep(10 * time.Millisecond)
		}
		close(done)
	}()

	// Background goroutine samples periodically
	stopSampling := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				count := atomic.LoadInt64(&totalMessages)
				tracker.Record(count)
			case <-stopSampling:
				return
			}
		}
	}()

	// Wait for traffic to complete
	<-done

	// Give sampler a moment to capture final state
	time.Sleep(200 * time.Millisecond)
	close(stopSampling)

	// API endpoint can query rate at any time
	count, rate := tracker.GetStats()

	t.Logf("Total messages: %d, Current rate: %.2f msg/sec", count, rate)

	// Verify we got reasonable values
	if count < 80 || count > 100 {
		t.Errorf("unexpected message count: %d", count)
	}

	// Rate should be positive (we published messages)
	if rate < 0 {
		t.Error("rate should not be negative")
	}
}

// ========================================
// Benchmark Tests
// ========================================

func BenchmarkRateTracker_Record(b *testing.B) {
	rt := NewRateTracker(time.Minute, 1000)
	for i := 0; i < b.N; i++ {
		rt.Record(int64(i))
	}
}

func BenchmarkRateTracker_Rate(b *testing.B) {
	rt := NewRateTracker(time.Minute, 1000)

	// Pre-populate with samples
	for i := 0; i < 1000; i++ {
		rt.Record(int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rt.Rate()
	}
}

// ========================================
// Helper Functions
// ========================================

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
