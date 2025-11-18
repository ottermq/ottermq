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

	if queue.maxLength == 0 {
		queue.mu.Unlock()
		return // No max length set
	}

	currentCount := uint32(queue.count)
	if currentCount < queue.maxLength {
		queue.mu.Unlock()
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

	// Pop all messages that need to be evicted while holding the lock
	evictedMessages := make([]Message, 0, excess)
	for evictedCount := range excess {
		oldest := queue.popUnlocked()
		if oldest == nil {
			log.Warn().
				Str("queue", queue.Name).
				Uint32("expected", excess).
				Uint32("evicted", evictedCount).
				Msg("Queue became empty during enforcement")
			break
		}
		evictedMessages = append(evictedMessages, *oldest)
	}

	// Release the queue lock BEFORE calling external functions that may acquire other locks
	// This prevents deadlock with vh.mu in handleDeadLetter
	queue.mu.Unlock()

	// Process evicted messages without holding queue.mu
	for _, msg := range evictedMessages {
		qll.vh.handleDeadLetter(queue, msg, REASON_MAX_LENGTH)
		qll.vh.deleteMessage(msg, queue)
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
