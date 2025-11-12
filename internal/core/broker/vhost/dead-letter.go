package vhost

import (
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
)

type DeadLetterer interface {
	DeadLetter(msg amqp.Message, queue *Queue, reason string) error
}

type NoOpDeadLetterer struct{}

func (d *NoOpDeadLetterer) DeadLetter(msg amqp.Message, queue *Queue, reason string) error {
	return nil
}

type DeadLetter struct {
	vh *VHost
}

func (dl *DeadLetter) DeadLetter(msg amqp.Message, queue *Queue, reason string) error {
	// 1. Add x-death header
	msg.Properties.Headers = dl.addXDeathHeader(msg.Properties.Headers, queue, reason)

	// 2. Determine routing key
	dlk := msg.RoutingKey
	if queue.Props.DeadLetterRoutingKey != "" {
		dlk = queue.Props.DeadLetterRoutingKey
	}

	// 3. Publish to DLX
	dlx := queue.Props.DeadLetterExchange
	_, err := dl.vh.Publish(dlx, dlk, &msg)

	return err
}

func (dl *DeadLetter) addXDeathHeader(headers map[string]any, queue *Queue, reason string) map[string]any {
	if headers == nil {
		headers = make(map[string]any)
	}

	// Create x-death entry
	death := map[string]any{
		"reason":       reason,
		"queue":        queue.Name,
		"exchange":     "placeholder-exchange",              // TODO: set actual exchange
		"routing-keys": []string{"placeholder-routing-key"}, // TODO: set actual routing key
		"time":         time.Now().UTC().Format(time.RFC3339),
		"count":        int64(1),
	}

	// Get existing x-death header
	xDeath, exists := headers["x-death"]
	if !exists {
		headers["x-death"] = []any{death}
	} else {
		deathArray := xDeath.([]any)

		for i, d := range deathArray {
			existingDeath := d.(map[string]any)
			if existingDeath["queue"] == queue.Name &&
				existingDeath["reason"] == reason {
				// Increment count
				existingDeath["count"] = existingDeath["count"].(int64) + 1
				deathArray[i] = existingDeath
				headers["x-death"] = deathArray
				return headers
			}
		}

		// No existing entry found, append new one
		deathArray = append([]any{death}, deathArray...)
		headers["x-death"] = deathArray
	}
	return headers
}
