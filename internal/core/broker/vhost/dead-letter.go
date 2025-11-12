package vhost

import (
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/rs/zerolog/log"
)

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

// xDeathEntry represents a single entry in the x-death header array of key-value pairs
type xDeathEntry map[string]any

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
	log.Debug().
		Str("queue", queue.Name).
		Str("dlx", queue.Props.DeadLetterExchange).
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
	if queue.Props.DeadLetterRoutingKey != "" {
		dlk = queue.Props.DeadLetterRoutingKey
		msg.RoutingKey = dlk
	}

	// 3. Publish to DLX
	dlx := queue.Props.DeadLetterExchange
	msg.Exchange = dlx
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
	if headers["x-first-death-queue"] == nil {
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
	death := xDeathEntry{
		"queue":               queue,
		"reason":              reason.String(),
		"count":               uint32(1), // long int
		"time":                time.Now().UTC().Format(time.RFC3339),
		"exchange":            exchange,
		"routing-keys":        routingKeys,
		"original-expiration": expiration,
	}

	// Get existing x-death header
	xDeath, exists := headers["x-death"]
	if !exists {
		// First x-death entry
		headers["x-death"] = []xDeathEntry{}
	}
	deathArray, ok := xDeath.([]xDeathEntry)
	if !ok {
		// Malformed x-death header, reset it
		log.Warn().Msg("Malformed x-death header, resetting it")
		headers["x-death"] = []xDeathEntry{}
	}
	death["count"] = uint32(len(deathArray)) + 1
	deathArray = append([]xDeathEntry{death}, deathArray...)
	headers["x-death"] = deathArray

	return headers
}
