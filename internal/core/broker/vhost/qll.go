package vhost

import "github.com/rs/zerolog/log"

// QueueLengthLimit (QLL) extension implementation

type QueueLengthLimiter interface {
	EnforceMaxLength(queue *Queue)
}

type NoOpQueueLengthLimiter struct{}

func (qll *NoOpQueueLengthLimiter) EnforceMaxLength(queue *Queue) {
	// No-op
}

type DefaultQueueLengthLimiter struct {
	vh *VHost
}

func (qll *DefaultQueueLengthLimiter) EnforceMaxLength(queue *Queue) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	if queue.maxLength == 0 {
		return // No max length set
	}

	currentCount := uint32(queue.count)
	if currentCount < queue.maxLength {
		return // Within limit - room for at least one more message
	}

	// Calculate how many messages to evict to make room for one more
	// If we're at limit (count == maxLength), evict 1
	// If over limit, evict (count - maxLength) + 1
	excess := (currentCount - queue.maxLength) + 1

	log.Debug().
		Str("queue", queue.Name).
		Uint32("current", currentCount).
		Uint32("max_length", queue.maxLength).
		Uint32("evicting", excess).
		Msg("Enforcing queue length limit")

	for i := range excess {
		// Remove oldest message
		oldest := queue.popUnlocked()
		if oldest == nil {
			log.Warn().
				Str("queue", queue.Name).
				Uint32("expected", excess).
				Uint32("evicted", i).
				Msg("Queue became empty during enforcement")
			break
		}
		qll.vh.handleDeadLetter(queue, *oldest, REASON_MAX_LENGTH)
		qll.vh.deleteMessage(*oldest, queue)
	}
}

func parseMaxLengthArgument(args map[string]interface{}) (uint32, bool) {
	maxLen, ok := args["x-max-length"]
	if !ok {
		return 0, false
	}

	value, ok := convertToPositiveInt64(maxLen)
	if ok && value > 0 {
		return uint32(value), true
	}
	return 0, false
}
