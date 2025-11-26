package models

type OverviewBrokerDetails struct {
	Product    string `json:"product"`
	Version    string `json:"version"`
	BuildInfo  string `json:"build_info"` // e.g., commit hash
	Platform   string `json:"platform"`
	GoVersion  string `json:"go_version"`
	UptimeSecs int    `json:"uptime_secs"`
	StartTime  string `json:"start_time"`
	DataDir    string `json:"data_dir"`
}

type OverviewNodeDetails struct {
	Name        string `json:"name"`
	MemoryUsage int    `json:"memory_usage"` // in bytes
	MemoryLimit int    `json:"memory_limit"` // in bytes
	FDUsed      int    `json:"fd_used"`
	FDTotal     int    `json:"fd_total"`
	DiskFree    int    `json:"disk_free"` // in bytes
	ProcUsed    int    `json:"proc_used"`
	SocketsUsed int    `json:"sockets_used"`
}

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

type BrokerConfigOverview struct {
	AMQPPort           string        `json:"amqp_port"`
	HTTPPort           string        `json:"http_port"`
	EnabledFeatures    []string      `json:"enabled_features"` // e.g., ["DLX", "TTL", "QLL"]
	ChannelMax         int           `json:"channel_max"`
	FrameMax           int           `json:"frame_max"`
	QueueBufferSize    int           `json:"queue_buffer_size"`
	PersistenceBackend string        `json:"persistence_backend"` // "json", "memento" (future)
	SSL                bool          `json:"ssl"`
	HTTPContexts       []HttpContext `json:"http_contexts"` // e.g., Context:path ["management:/", "swagger:/", "prometheus:/"]
}

type HttpContext struct {
	Name string `json:"name"`
	Port string `json:"port"`
	Path string `json:"path"`
}
