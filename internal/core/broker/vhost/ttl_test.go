package vhost

import (
	"strconv"
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
)

func TestDefaultTTLManager_CheckExpiration_PerMessageTTL(t *testing.T) {
	ttlManager := &DefaultTTLManager{}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: make(QueueArgs),
		},
	}

	// Message with 100ms TTL that has expired
	expiredTime := time.Now().Add(-200 * time.Millisecond).UnixMilli()
	expiredMsg := &Message{
		ID:         "expired-msg",
		EnqueuedAt: time.UnixMilli(expiredTime),
		Properties: amqp.BasicProperties{
			Expiration: strconv.FormatInt(time.Now().Add(-100*time.Millisecond).UnixMilli(), 10),
		},
	}

	expired, err := ttlManager.CheckExpiration(expiredMsg, queue)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !expired {
		t.Error("Expected message to be expired")
	}

	// Message with 1 hour TTL that has NOT expired
	futureTime := time.Now().Add(1 * time.Hour).UnixMilli()
	freshMsg := &Message{
		ID:         "fresh-msg",
		EnqueuedAt: time.Now(),
		Properties: amqp.BasicProperties{
			Expiration: strconv.FormatInt(futureTime, 10),
		},
	}

	expired, err = ttlManager.CheckExpiration(freshMsg, queue)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if expired {
		t.Error("Expected message to NOT be expired")
	}
}

func TestDefaultTTLManager_CheckExpiration_PerQueueTTL(t *testing.T) {
	ttlManager := &DefaultTTLManager{}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: QueueArgs{
				"x-message-ttl": int64(100), // 100ms queue TTL
			},
		},
	}

	// Message enqueued 200ms ago (expired)
	expiredMsg := &Message{
		ID:         "expired-msg",
		EnqueuedAt: time.Now().Add(-200 * time.Millisecond),
		Properties: amqp.BasicProperties{},
	}

	expired, err := ttlManager.CheckExpiration(expiredMsg, queue)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !expired {
		t.Error("Expected message to be expired based on queue TTL")
	}

	// Message enqueued 50ms ago (NOT expired)
	freshMsg := &Message{
		ID:         "fresh-msg",
		EnqueuedAt: time.Now().Add(-50 * time.Millisecond),
		Properties: amqp.BasicProperties{},
	}

	expired, err = ttlManager.CheckExpiration(freshMsg, queue)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if expired {
		t.Error("Expected message to NOT be expired")
	}
}

func TestDefaultTTLManager_CheckExpiration_PerMessageOverridesQueue(t *testing.T) {
	ttlManager := &DefaultTTLManager{}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: QueueArgs{
				"x-message-ttl": int64(1000), // 1 second queue TTL
			},
		},
	}

	// Message with 100ms per-message TTL (should take precedence over queue TTL)
	// Enqueued 200ms ago, so expired by per-message TTL
	expiredTime := time.Now().Add(-200 * time.Millisecond).UnixMilli()
	msg := &Message{
		ID:         "msg",
		EnqueuedAt: time.UnixMilli(expiredTime),
		Properties: amqp.BasicProperties{
			Expiration: "100", // 100ms per-message TTL
		},
	}

	expired, err := ttlManager.CheckExpiration(msg, queue)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !expired {
		t.Error("Expected message to be expired by per-message TTL (should override queue TTL)")
	}
}

func TestDefaultTTLManager_CheckExpiration_NoTTL(t *testing.T) {
	ttlManager := &DefaultTTLManager{}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: make(QueueArgs),
		},
	}

	msg := &Message{
		ID:         "msg",
		EnqueuedAt: time.Now().Add(-10 * time.Hour), // Very old message
		Properties: amqp.BasicProperties{},
	}

	expired, err := ttlManager.CheckExpiration(msg, queue)

	if err != ErrNoTTLConfigured {
		t.Errorf("Expected ErrNoTTLConfigured, got: %v", err)
	}
	if expired {
		t.Error("Expected message to NOT be expired when no TTL configured")
	}
}

func TestDefaultTTLManager_CheckExpiration_InvalidExpiration(t *testing.T) {
	ttlManager := &DefaultTTLManager{}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: make(QueueArgs),
		},
	}

	// Message with invalid (non-numeric) expiration
	msg := &Message{
		ID:         "msg",
		EnqueuedAt: time.Now(),
		Properties: amqp.BasicProperties{
			Expiration: "invalid-number",
		},
	}

	expired, err := ttlManager.CheckExpiration(msg, queue)

	// Should fall back to checking queue TTL, which doesn't exist
	if err != ErrNoTTLConfigured {
		t.Errorf("Expected ErrNoTTLConfigured for invalid expiration, got: %v", err)
	}
	if expired {
		t.Error("Expected message to NOT be expired")
	}
}

func TestDefaultTTLManager_CheckExpiration_ZeroExpiration(t *testing.T) {
	ttlManager := &DefaultTTLManager{}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: make(QueueArgs),
		},
	}

	// Message with zero expiration (should be treated as no TTL)
	msg := &Message{
		ID:         "msg",
		EnqueuedAt: time.Now(),
		Properties: amqp.BasicProperties{
			Expiration: "0",
		},
	}

	expired, err := ttlManager.CheckExpiration(msg, queue)

	if err != ErrNoTTLConfigured {
		t.Errorf("Expected ErrNoTTLConfigured for zero expiration, got: %v", err)
	}
	if expired {
		t.Error("Expected message to NOT be expired with zero TTL")
	}
}

func TestNoOpTTLManager_CheckExpiration(t *testing.T) {
	ttlManager := &NoOpTTLManager{}
	queue := &Queue{
		Name: "test-queue",
		Props: &QueueProperties{
			Arguments: QueueArgs{
				"x-message-ttl": int64(100),
			},
		},
	}

	// Even with expired TTL, NoOp should return not expired
	expiredMsg := &Message{
		ID:         "expired-msg",
		EnqueuedAt: time.Now().Add(-1 * time.Hour),
		Properties: amqp.BasicProperties{
			Expiration: strconv.FormatInt(1800000, 10), // 30 minutes in ms
		},
	}

	expired, err := ttlManager.CheckExpiration(expiredMsg, queue)

	if err != nil {
		t.Errorf("Expected no error from NoOp TTL manager, got: %v", err)
	}
	if expired {
		t.Error("NoOp TTL manager should never expire messages")
	}
}

func TestParseTTLArgument_ValidInt64(t *testing.T) {
	args := map[string]interface{}{
		"x-message-ttl": int64(5000),
	}

	ttl, ok := parseTTLArgument(args)
	if !ok {
		t.Error("Expected TTL to be parsed successfully")
	}
	if ttl != 5000 {
		t.Errorf("Expected TTL 5000, got %d", ttl)
	}
}

func TestParseTTLArgument_ValidInt32(t *testing.T) {
	args := map[string]interface{}{
		"x-message-ttl": int32(3000),
	}

	ttl, ok := parseTTLArgument(args)
	if !ok {
		t.Error("Expected TTL to be parsed successfully")
	}
	if ttl != 3000 {
		t.Errorf("Expected TTL 3000, got %d", ttl)
	}
}

func TestParseTTLArgument_ValidInt(t *testing.T) {
	args := map[string]interface{}{
		"x-message-ttl": int(2000),
	}

	ttl, ok := parseTTLArgument(args)
	if !ok {
		t.Error("Expected TTL to be parsed successfully")
	}
	if ttl != 2000 {
		t.Errorf("Expected TTL 2000, got %d", ttl)
	}
}

func TestParseTTLArgument_Missing(t *testing.T) {
	args := map[string]interface{}{
		"other-arg": "value",
	}

	_, ok := parseTTLArgument(args)
	if ok {
		t.Error("Expected TTL parsing to fail when x-message-ttl is missing")
	}
}

func TestParseTTLArgument_InvalidType(t *testing.T) {
	args := map[string]interface{}{
		"x-message-ttl": "not-a-number",
	}

	_, ok := parseTTLArgument(args)
	if ok {
		t.Error("Expected TTL parsing to fail for invalid type")
	}
}

func TestParseTTLArgument_NegativeValue(t *testing.T) {
	args := map[string]interface{}{
		"x-message-ttl": int64(-1000),
	}

	_, ok := parseTTLArgument(args)
	if ok {
		t.Error("Expected TTL parsing to fail for negative value")
	}
}
