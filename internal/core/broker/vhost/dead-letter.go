package vhost

import (
	"errors"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/rs/zerolog/log"
)

var ErrNoDLXConfigured = errors.New("queue has no dead-letter-exchange configured")

type ReasonType string

const (
	REASON_REJECTED       ReasonType = "rejected"       // basic.reject or basic.nack with requeue false
	REASON_EXPIRED        ReasonType = "expired"        // message TTL expired
	REASON_MAX_LENGTH     ReasonType = "maxlen"         // message length exceeded
	REASON_DELIVERY_LIMIT ReasonType = "delivery_limit" // delivery limit exceeded (quorum queues -- not implemented yet)
)

func (r ReasonType) String() string {
	return string(r)
}

type DeadLetterer interface {
	DeadLetter(msg amqp.Message, queue *Queue, reason ReasonType) error
}

type NoOpDeadLetterer struct{}

func (d *NoOpDeadLetterer) DeadLetter(msg amqp.Message, queue *Queue, reason ReasonType) error {
	return nil
}

type DeadLetter struct {
	vh *VHost
}

func (dl *DeadLetter) DeadLetter(msg amqp.Message, queue *Queue, reason ReasonType) error {
	// Check if queue has DLX configured
	dlx, ok := queue.Props.Arguments["x-dead-letter-exchange"].(string)
	if !ok || dlx == "" {
		return ErrNoDLXConfigured
	}

	log.Debug().
		Str("queue", queue.Name).
		Str("dlx", dlx).
		Str("reason", reason.String()).
		Msg("Dead-lettering message")
	// 1. Add x-death header
	headers := msg.Properties.Headers
	exchange := msg.Exchange
	routingKey := []string{msg.RoutingKey}
	// TODO: create a feature flag to enable/disable support for CC and BCC headers
	routingKey = appendCCBCCToRoutingKey(headers, routingKey)
	expiration := msg.Properties.Expiration
	msg.Properties.Expiration = "" // Clear expiration on dead-lettered message
	msg.Properties.Headers = dl.addXDeathHeader(headers, expiration, exchange, queue.Name, routingKey, reason)

	// 2. Determine routing key
	dlk := msg.RoutingKey
	if dlrk, ok := queue.Props.Arguments["x-dead-letter-routing-key"].(string); ok && dlrk != "" {
		dlk = dlrk
		msg.RoutingKey = dlk
	}

	// 3. Publish to DLX
	msg.Exchange = dlx
	log.Info().
		Str("dlx", dlx).
		Str("routing_key", dlk).
		Int("body_len", len(msg.Body)).
		Int("headers_count", len(msg.Properties.Headers)).
		Msg("=== PUBLISHING TO DLX ===")
	_, err := dl.vh.Publish(dlx, dlk, &msg)

	return err
}

// appendCCBCCToRoutingKey appends CC and BCC headers to the routing key slice
func appendCCBCCToRoutingKey(headers map[string]any, routingKey []string) []string {
	if cc, ok := headers["CC"]; ok {
		if ccArray, ok := cc.([]string); ok && len(ccArray) > 0 {
			routingKey = append(routingKey, ccArray...)
		}
	}
	if bcc, ok := headers["BCC"]; ok {
		if bccArray, ok := bcc.([]string); ok && len(bccArray) > 0 {
			routingKey = append(routingKey, bccArray...)
		}
	}
	return routingKey
}

// addXDeathHeader adds or updates the x-death header in the message properties
// expiration is the original expiration value of the message (Not yet implemented on the broker)
func (dl *DeadLetter) addXDeathHeader(headers map[string]any, expiration, exchange, queue string, routingKeys []string, reason ReasonType) map[string]any {
	if headers == nil {
		headers = make(map[string]any)
	}
	if _, exists := headers["x-first-death-queue"]; !exists {
		// first time dead-lettering, initialize x-first-death-* headers
		headers["x-first-death-queue"] = queue
		headers["x-first-death-reason"] = reason.String()
		headers["x-first-death-exchange"] = exchange
	}
	// update x-last-death-* headers
	headers["x-last-death-queue"] = queue
	headers["x-last-death-reason"] = reason.String()
	headers["x-last-death-exchange"] = exchange

	// Create x-death entry
	death := map[string]any{
		"queue":        queue,
		"reason":       reason.String(),
		"time":         time.Now().UTC().Format(time.RFC3339),
		"exchange":     exchange,
		"routing-keys": routingKeys,
		// "original-expiration": expiration,
	}

	// Get existing x-death header
	xDeathEvents, ok := headers["x-death"].([]map[string]any)
	if !ok {
		xDeathEvents = []map[string]any{}
	}
	death["count"] = int64(len(xDeathEvents) + 1) // Must be int64 for proper AMQP encoding
	xDeathEvents = append([]map[string]any{death}, xDeathEvents...)
	headers["x-death"] = xDeathEvents
	return headers
}
