package models

import (
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/pkg/metrics"
)

type ConnectionInfoDTO struct {
	VHostName     string    `json:"vhost"`
	Name          string    `json:"name"` // ip
	Username      string    `json:"user_name"`
	State         string    `json:"state"` // "disconnected" or "running"
	SSL           bool      `json:"ssl"`
	Protocol      string    `json:"protocol"`
	Channels      int       `json:"channels"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	ConnectedAt   time.Time `json:"connected_at"`
}

func MapListConnectionsDTO(connections []amqp.ConnectionInfo) []ConnectionInfoDTO {
	listConnectonsDTO := make([]ConnectionInfoDTO, len(connections))
	for i, connection := range connections {
		listConnectonsDTO[i] = MapConnectionInfoDTO(connection)
	}
	return listConnectonsDTO
}

func MapConnectionInfoDTO(connection amqp.ConnectionInfo) ConnectionInfoDTO {
	state := "disconnected"
	if connection.Client.Ctx.Err() == nil {
		state = "running"
	}
	channels := len(connection.Channels)
	connInfo := ConnectionInfoDTO{
		VHostName:     connection.VHostName,
		Name:          connection.Client.RemoteAddr,
		Username:      connection.Client.Config.Username,
		State:         state,
		SSL:           connection.Client.Config.SSL,
		Protocol:      connection.Client.Config.Protocol,
		Channels:      channels,
		LastHeartbeat: connection.Client.LastHeartbeat,
		ConnectedAt:   connection.Client.ConnectedAt,
	}
	return connInfo
}

// VHostDTO represents a virtual host.
type VHostDTO struct {
	Name             string   `json:"name"`
	Users            []string `json:"users,omitempty"` // list of users with access to this vhost
	State            string   `json:"state"`           // "running", "idle"
	UnconfirmedCount int      `json:"unconfirmed_count"`
	PrefetchCount    uint16   `json:"prefetch_count"`
	UnackedCount     int      `json:"unacked_count"`
}

type QueueDTO struct {
	// Identity
	VHost string `json:"vhost"`
	Name  string `json:"name"`

	// Message counts (RabbitMQ compatible field names)
	Messages           int `json:"messages"`       // Ready
	MessagesReady      int `json:"messages_ready"` // Alias
	MessagesUnacked    int `json:"messages_unacked"`
	MessagesPersistent int `json:"messages_persistent"`
	MessagesTotal      int `json:"messages_total"` // Ready + Unacked

	// Consumers stats
	Consumers       int `json:"consumers"`
	ConsumersActive int `json:"consumers_active"`

	// Properties/flags
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Exclusive  bool           `json:"exclusive"`
	Arguments  map[string]any `json:"arguments,omitempty"`

	// DLX Configuration (extracted for convenience)
	DeadLetterExchange   *string `json:"dead_letter_exchange,omitempty"`
	DeadLetterRoutingKey *string `json:"dead_letter_routing_key,omitempty"`

	// TTL Configuration
	MessageTTL *int64 `json:"message_ttl,omitempty"` // in milliseconds

	// Queue Length Limit (QLL AKA Max Length)
	MaxLength *int32 `json:"max_length,omitempty"`

	// Priority queue configuration
	MaxPriority *uint8 `json:"max_priority,omitempty"` // x-max-priority

	// State
	State string `json:"state"` // "running", "idle", "flow"

	// Metadata
	OwnerConnection    string `json:"owner_connection"` // for exclusive queues
	PersistenceEnabled bool   `json:"persistence_enabled"`
}

type ExchangeDTO struct {
	// Identity
	VHost string `json:"vhost"`
	Name  string `json:"name"`
	Type  string `json:"type"`

	// Properties/flags
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Internal   bool           `json:"internal"`
	Arguments  map[string]any `json:"arguments,omitempty"`

	// Stats
	MessageStatsIn  *MessageStats `json:"message_stats_in,omitempty"`
	MessageStatsOut *MessageStats `json:"message_stats_out,omitempty"`
}

type MessageStats struct {
	PublishCount int     `json:"publish"`
	PublishRate  float64 `json:"publish_details.rate"`
	DeliverCount int64   `json:"deliver_get"`
	DeliverRate  float64 `json:"deliver_get_details.rate"`
}

type ConsumerDTO struct {
	ConsumerTag    string         `json:"consumer_tag"`
	QueueName      string         `json:"queue_name"`
	ChannelDetails ChannelInfoDTO `json:"channel_details"`
	AckRequired    bool           `json:"ack_required"` // !NoAck
	Exclusive      bool           `json:"exclusive"`
	PrefetchCount  int            `json:"prefetch_count"`
	Active         bool           `json:"active"`
	Arguments      map[string]any `json:"arguments,omitempty"`
}

type ChannelInfoDTO struct {
	Number         uint16 `json:"number"` // channel number
	ConnectionName string `json:"connection_name"`
	User           string `json:"user"`
}

type ChannelDetailDTO struct {
	Number         uint16 `json:"number"` // channel number
	ConnectionName string `json:"connection_name"`
	VHost          string `json:"vhost"`
	User           string `json:"user"`
	State          string `json:"state"` // "running", "flow", "closing"
	// Details
	UnconfirmedCount int    `json:"unconfirmed_count"`
	PrefetchCount    uint16 `json:"prefetch_count"`
	UnackedCount     int    `json:"unacked_count"`
	// Stats
	PublishRate float64 `json:"publish_rate"`
	ConfirmRate float64 `json:"confirm_rate"`
	DeliverRate float64 `json:"deliver_rate"`
	AckRate     float64 `json:"ack_rate"`
}

type BindingDTO struct {
	// Source = Exchange name
	Source string `json:"source"` // Exchange name
	VHost  string `json:"vhost"`
	// Destination = Queue name (or Exchange name if exchange-to-exchange binding )
	Destination     string         `json:"destination"`
	DestinationType string         `json:"destination_type"` // "queue" or "exchange"
	RoutingKey      string         `json:"routing_key"`
	Arguments       map[string]any `json:"arguments,omitempty"`
	PropertiesKey   string         `json:"properties_key"` // Hash for idempotency
}

type OverviewDTO struct {
	BrokerDetails   OverviewBrokerDetails   `json:"broker"`
	NodeDetails     OverviewNodeDetails     `json:"node"`
	ObjectTotals    OverviewObjectTotals    `json:"object_totals"`
	MessageStats    OverviewMessageStats    `json:"message_stats"`
	ConnectionStats OverviewConnectionStats `json:"connection_stats"`
	Configuration   BrokerConfigOverview    `json:"configuration"`
	Metrics         metrics.BrokerSnapshot  `json:"metrics"`
}

type MessageDTO struct {
	ID          string         `json:"id"`
	Payload     []byte         `json:"payload"`
	Properties  map[string]any `json:"properties"`
	DeliveryTag uint64         `json:"delivery_tag"`
	Redelivered bool           `json:"redelivered"`
}

// ChannelInfo represents information about a channel.
type ChannelInfo struct {
	Number           uint16 `json:"number"` // channel number
	ConnectionName   string `json:"connection_name"`
	VHost            string `json:"vhost"`
	User             string `json:"user"`
	State            string `json:"state"` // "running", "flow", "closing"
	UnconfirmedCount int    `json:"unconfirmed_count"`
	PrefetchCount    uint16 `json:"prefetch_count"`
	UnackedCount     int    `json:"unacked_count"`
	InTransaction    bool   `json:"in_transaction"`
	ConfirmMode      bool   `json:"confirm_mode"`
}
