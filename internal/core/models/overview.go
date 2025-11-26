package models

type OverviewObjectTotals struct {
	Connections int `json:"connections"`
	Channels    int `json:"channels"`
	Exchanges   int `json:"exchanges"`
	Queues      int `json:"queues"`
	Consumers   int `json:"consumers"`
}

type OverviewMessageStats struct {
	// Current state (easy to calculate now)
	MessagesReady   int `json:"messages_ready"`          // Sum of all queue.Len()
	MessagesUnacked int `json:"messages_unacknowledged"` // Sum of unacked
	MessagesTotal   int `json:"messages_total"`          // Ready + Unacked

	// Per-queue breakdown
	QueueStats []QueueMessageBreakdown `json:"queue_stats"`
}

type QueueMessageBreakdown struct {
	QueueName       string `json:"name"`
	VHost           string `json:"vhost"`
	MessagesReady   int    `json:"messages"`
	MessagesUnacked int    `json:"messages_unacknowledged"`
}

type OverviewConnectionStats struct {
	Total             int `json:"total"`
	ClientConnections int `json:"client_connections"`

	// By state
	Running int `json:"running"`
	Closing int `json:"closing"`

	// Protocol breakdown
	AMQP091 int `json:"amqp091"`
}

type OverviewFeatureUsage struct {
	QueuesWithDLX         int `json:"queues_with_dlx"`
	QueuesWithTTL         int `json:"queues_with_ttl"`
	QueuesWithQLL         int `json:"queues_with_qll"`
	TransactionalChannels int `json:"transactional_channels"`
	ChannelsWithQoS       int `json:"channels_with_qos"`
}

type OverviewLimits struct {
	MaxConnections     int    `json:"max_connections"`
	QueueBufferSize    int    `json:"queue_buffer_size"`
	PersistenceBackend string `json:"persistence_backend"` // "json", "memento" (future)
}
