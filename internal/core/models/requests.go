package models

import "time"

type CreateQueueRequest struct {
	// Properties/flags
	Passive    bool           `json:"passive" default:"false"`
	Durable    bool           `json:"durable" default:"false"`
	AutoDelete bool           `json:"auto_delete" default:"false"`
	Arguments  map[string]any `json:"arguments,omitempty"`

	// Convenience fields (auto-mapped to arguments)
	MaxLength            *int32  `json:"max_length,omitempty"`
	MessageTTL           *int64  `json:"message_ttl,omitempty"`
	DeadLetterExchange   *string `json:"x-dead-letter-exchange,omitempty"`
	DeadLetterRoutingKey *string `json:"x-dead-letter-routing-key,omitempty"`
}

type CreateExchangeRequest struct {
	ExchangeType string `json:"type" validate:"required,oneof=direct fanout topic headers"`

	// Properties
	Passive    bool           `json:"passive"`
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	NoWait     bool           `json:"no_wait"` // not needed for management API. Included for completeness
	Arguments  map[string]any `json:"arguments,omitempty"`
}

type CreateBindingRequest struct {
	VHost       string         `json:"vhost"`                           // Optional; defaults to "/"
	Source      string         `json:"source" validate:"required"`      // Exchange
	Destination string         `json:"destination" validate:"required"` // Queue (?or Exchange too??)
	RoutingKey  string         `json:"routing_key"`
	Arguments   map[string]any `json:"arguments,omitempty"`
}

type DeleteBindingRequest struct {
	VHost       string         `json:"vhost"`                           // Optional; defaults to "/"
	Source      string         `json:"source" validate:"required"`      // Exchange
	Destination string         `json:"destination" validate:"required"` // Queue (?or Exchange too??)
	RoutingKey  string         `json:"routing_key"`
	Arguments   map[string]any `json:"arguments,omitempty"`
}

type PublishMessageRequest struct {
	VHost        string `json:"vhost"` // Optional; defaults to "/"
	ExchangeName string `json:"exchange" validate:"required"`
	RoutingKey   string `json:"routing_key"`
	Payload      string `json:"payload" validate:"required"`

	// Message properties (AMQP 0-9-1 spec)
	ContentType     string         `json:"content_type"`
	ContentEncoding string         `json:"content_encoding"`
	DeliveryMode    uint8          `json:"delivery_mode"` // 1=transient, 2=persistent
	Priority        uint8          `json:"priority"`
	CorrelationId   string         `json:"correlation_id"`
	ReplyTo         string         `json:"reply_to"`
	Expiration      string         `json:"expiration"` // TTL in milliseconds as string
	MessageID       string         `json:"message_id"`
	Timestamp       *time.Time     `json:"timestamp"`
	Type            string         `json:"type"`
	UserId          string         `json:"user_id"`
	AppId           string         `json:"app_id"`
	Headers         map[string]any `json:"headers,omitempty"`

	// Routing flags
	Mandatory bool `json:"mandatory"`
	Immediate bool `json:"immediate"` // Deprecated in AMQP 0-9-1 (included for compatibility)
}

type GetMessageRequest struct {
	AckMode      AckType `json:"ack_mode"` // "ack", "no_ack", "reject"
	MessageCount int     `json:"message_count"`
}

type AckType string

const (
	Ack           AckType = "ack"            // automatic ack
	NoAck         AckType = "no_ack"         // no ack, requeue=ttrue
	Reject        AckType = "reject"         // reject, requeue=false
	RejectRequeue AckType = "reject_requeue" // reject, requeue=true
)
