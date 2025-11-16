package vhost

import "errors"

var ErrNoTTLConfigured = errors.New("queue has no TTL configured")

type TTLManager interface {
	StartExpirationSweeper(queue *Queue)
	StopExpirationSweeper(queue *Queue)
	CheckExpiration(msg *Message, queue *Queue) (bool, error)
}
type DefaultTTLManager struct {
	vh *VHost
}

type NoOpTTLManager struct{}

func (tm *NoOpTTLManager) StartExpirationSweeper(queue *Queue) {
	// No-op
}

func (tm *NoOpTTLManager) StopExpirationSweeper(queue *Queue) {
	// No-op
}

func (tm *NoOpTTLManager) CheckExpiration(msg *Message, queue *Queue) (bool, error) {
	// No-op, always return false
	return false, nil
}

func (dtm *DefaultTTLManager) StartExpirationSweeper(queue *Queue) {
	// Implementation of expiration sweeper
	// This could be a goroutine that periodically checks for expired messages
}

func (dtm *DefaultTTLManager) StopExpirationSweeper(queue *Queue) {
	// Implementation to stop the expiration sweeper
}

func (dtm *DefaultTTLManager) CheckExpiration(msg *Message, queue *Queue) (bool, error) {
	panic("not implemented")
}

func parseTTLArgument(args map[string]interface{}) (int64, bool) {
	ttlFloat, ok := args["x-message-ttl"].(float64)
	if !ok {
		return 0, false
	}
	return int64(ttlFloat), true
}
