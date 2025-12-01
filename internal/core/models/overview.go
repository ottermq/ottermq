package models

type OverviewBrokerDetails struct {
	Product      string `json:"product"`
	Version      string `json:"version"`
	CommitNumber string `json:"commit_number"`
	CommitHash   string `json:"commit_hash"`
	Platform     string `json:"platform"`
	GoVersion    string `json:"go_version"`
	UptimeSecs   int    `json:"uptime_secs"`
	StartTime    string `json:"start_time"`
	DataDir      string `json:"data_dir"`
}

type CommitInfo struct {
	Version    string `json:"version"`
	CommitNum  string `json:"commit_num"`
	CommitHash string `json:"commit_hash"`
}

type OverviewNodeDetails struct {
	Name            string   `json:"name"`
	FDUsed          int      `json:"fd_used"`
	FDLimit         int      `json:"fd_limit"`
	Goroutines      int      `json:"goroutines"`
	GoroutinesLimit int      `json:"goroutines_limit"`
	MemoryUsage     int      `json:"memory_usage"`   // in bytes
	MemoryLimit     int      `json:"memory_limit"`   // in bytes
	DiskAvailable   int      `json:"disk_available"` // in bytes
	DiskTotal       int      `json:"disk_total"`     // in bytes
	UptimeSecs      int      `json:"uptime_secs"`
	Cores           int      `json:"cores"`
	Info            NodeInfo `json:"info"`
}

type NodeInfo struct {
	MessageRates   string   `json:"message_rates"` // message rates strategy
	EnabledPlugins []string `json:"enabled_plugins"`
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
	ClientConnections int `json:"client_connections"` // Excludes management connections

	// By state
	Running int `json:"running"`
	Closing int `json:"closing"`

	// Protocol breakdown
	AMQP091 int `json:"amqp091"`
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
	Name    string `json:"name"`
	BoundTo string `json:"bound_to"`
	Port    string `json:"port"`
	SSL     bool   `json:"ssl"`
	Path    string `json:"path"`
}
