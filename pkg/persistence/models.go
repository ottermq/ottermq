package persistence

// Universal models that all implementations can use
type MessageProperties struct {
	ContentType     string         `json:"content_type,omitempty"`
	ContentEncoding string         `json:"content_encoding,omitempty"`
	Headers         map[string]any `json:"headers,omitempty"`
	DeliveryMode    uint8          `json:"delivery_mode,omitempty"`
	Priority        uint8          `json:"priority,omitempty"`
	CorrelationID   string         `json:"correlation_id,omitempty"`
	ReplyTo         string         `json:"reply_to,omitempty"`
	Expiration      string         `json:"expiration,omitempty"`
	MessageID       string         `json:"message_id,omitempty"`
	Timestamp       int64          `json:"timestamp,omitempty"`
	Type            string         `json:"type,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
	AppID           string         `json:"app_id,omitempty"`
}

// Message represents a stored message (universal across implementations)
type Message struct {
	ID         string            `json:"id"`
	Body       []byte            `json:"body"`
	Properties MessageProperties `json:"properties"`
	EnqueuedAt int64             `json:"enqueued_at"`
}

// Basic queue/exchange properties - universal concepts
type QueueProperties struct {
	Durable    bool           `json:"durable"`
	Exclusive  bool           `json:"exclusive"`
	AutoDelete bool           `json:"auto_delete"`
	Arguments  map[string]any `json:"arguments"`
}

type ExchangeProperties struct {
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Internal   bool           `json:"internal"`
	Arguments  map[string]any `json:"arguments"`
}

type BindingData struct {
	QueueName  string         `json:"queue_name"`
	RoutingKey string         `json:"routing_key"`
	Arguments  map[string]any `json:"arguments"`
}

type ExchangeSnapshot struct {
	Name       string
	Type       string
	Properties ExchangeProperties
	Bindings   []BindingData
}

type QueueSnapshot struct {
	Name       string
	Properties QueueProperties
	Messages   []Message
}

// * Passive and NoWait are not needed in persistence
// because they are only relevant during declaration time
// Even though they are included here for completeness,
// and it could make sense to persist them if we decide to
// track declaration history
