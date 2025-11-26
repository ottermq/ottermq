package models

import (
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
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
		state := "disconnected"
		if connection.Client.Ctx.Err() == nil {
			state = "running"
		}
		channels := len(connection.Channels)
		listConnectonsDTO[i] = ConnectionInfoDTO{
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
	}
	return listConnectonsDTO
}

type ChannelDTO struct {
}

type VHostDTO struct {
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
	Arguments  map[string]any `json:"arguments_count"`

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
	Arguments  map[string]any `json:"arguments_count"`

	// DLX Configuration (extracted for convenience)
	DeadLetterExchange   *string `json:"dead_letter_exchange,omitempty"`
	DeadLetterRoutingKey *string `json:"dead_letter_routing_key,omitempty"`

	// TTL Configuration
	MessageTTL *int64 `json:"message_ttl,omitempty"` // in milliseconds

	// Queue Length Limit (QLL AKA Max Length)
	MaxLength *int32 `json:"max_length,omitempty"`

	// State
	State string `json:"state"` // "running", "idle", "flow"

	// Metadata
	OwnerConnection    string `json:"owner_connection"` // for exclusive queues
	PersistenceEnabled bool   `json:"persistence_enabled"`
}

type BindingDTO struct {
	VHostName string              `json:"vhost"`
	VHostId   string              `json:"vhost_id"`
	Exchange  string              `json:"exchange"`
	Bindings  map[string][]string `json:"bindings"`
}

type ConsumerDTO struct {
}

type OverviewDTO struct {
}
