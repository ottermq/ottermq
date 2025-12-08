package metrics

import (
	"sync"
	"time"
)

type RateTracker struct {
	mu         sync.RWMutex
	samples    []Sample
	windowSize time.Duration
	maxSamples int
}

type Sample struct {
	Count     int64
	Timestamp time.Time
}

func NewRateTracker(windowSize time.Duration, maxSamples int) *RateTracker {
	return &RateTracker{
		samples:    make([]Sample, 0, maxSamples),
		windowSize: windowSize,
		maxSamples: maxSamples,
	}
}

func (rt *RateTracker) Record(totalCount int64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now()

	// add new sample
	rt.samples = append(rt.samples, Sample{
		Count:     totalCount,
		Timestamp: now,
	})

	// Prune samples outside the window
	cutoff := now.Add(-rt.windowSize)
	validIdx := 0
	for i, sample := range rt.samples {
		if sample.Timestamp.After(cutoff) {
			validIdx = i
			break
		}
	}

	if validIdx > 0 {
		copy(rt.samples, rt.samples[validIdx:])
		rt.samples = rt.samples[:len(rt.samples)-validIdx]
	}

	// Cap samples to maxSamples to prevent unbounded growth
	if len(rt.samples) > rt.maxSamples {
		excess := len(rt.samples) - rt.maxSamples
		copy(rt.samples, rt.samples[excess:])
		rt.samples = rt.samples[:rt.maxSamples]
	}
}

// Rate computes the rate of change per second based on the recorded samples.
func (rt *RateTracker) Rate() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if len(rt.samples) < 2 {
		return 0.0
	}

	oldest := rt.samples[0]
	newest := rt.samples[len(rt.samples)-1]

	elapsed := newest.Timestamp.Sub(oldest.Timestamp).Seconds()
	if elapsed <= 0 {
		return 0.0
	}

	countDelta := newest.Count - oldest.Count
	return float64(countDelta) / elapsed
}

func (rt *RateTracker) GetStats() (count int64, rate float64) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if len(rt.samples) == 0 {
		return 0, 0.0
	}

	newest := rt.samples[len(rt.samples)-1]
	count = newest.Count

	if len(rt.samples) >= 2 {
		oldest := rt.samples[0]
		elapsed := newest.Timestamp.Sub(oldest.Timestamp).Seconds()
		if elapsed > 0 {
			rate = float64(newest.Count-oldest.Count) / elapsed
		}
	}

	return count, rate
}

func (rt *RateTracker) GetSamples() []Sample {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	samplesCopy := make([]Sample, len(rt.samples))
	copy(samplesCopy, rt.samples)
	return samplesCopy
}

func (rt *RateTracker) Clear() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.samples = rt.samples[:0]
}
