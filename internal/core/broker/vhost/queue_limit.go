package vhost

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

	concurrentCount := uint32(queue.count)
	if concurrentCount <= queue.maxLength {
		return // Within limit
	}
	excess := concurrentCount - queue.maxLength
	for range excess {
		// Remove oldest message
		oldest := queue.popUnlocked()
		if oldest == nil {
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
