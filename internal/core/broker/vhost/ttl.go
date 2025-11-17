package vhost

import (
	"errors"
	"strconv"
	"time"
)

var ErrNoTTLConfigured = errors.New("queue has no TTL configured")

type TTLManager interface {
	CheckExpiration(msg *Message, queue *Queue) (bool, error)
}
type DefaultTTLManager struct {
	vh *VHost
}

type NoOpTTLManager struct{}

func (tm *NoOpTTLManager) CheckExpiration(msg *Message, queue *Queue) (bool, error) {
	// No-op, always return false
	return false, nil
}

func (dtm *DefaultTTLManager) CheckExpiration(msg *Message, queue *Queue) (bool, error) {
	// Per-message TTL (Expiration property) takes precedence
	if msg.Properties.Expiration != "" {
		expirationMs, err := strconv.ParseInt(msg.Properties.Expiration, 10, 64)
		if err == nil && expirationMs > 0 {
			if time.Now().UnixMilli() >= expirationMs {
				return true, nil
			}
			return false, nil
		}
	}

	// Fall back to per-queue TTL (x-message-ttl)
	if ttlMs, ok := parseTTLArgument(queue.Props.Arguments); ok {
		age := time.Since(msg.EnqueuedAt).Milliseconds()
		if age >= ttlMs {
			return true, nil
		}
		return false, nil
	}

	return false, ErrNoTTLConfigured
}

// parseTTLArgument extracts the x-message-ttl argument from the queue arguments
func parseTTLArgument(args map[string]interface{}) (int64, bool) {
	ttl, ok := args["x-message-ttl"]
	if !ok {
		return 0, false
	}

	// Handle different integer types that may come from AMQP
	value, ok := convertToPositiveInt64(ttl)
	if ok {
		return value, ok
	}
	return 0, false
}
