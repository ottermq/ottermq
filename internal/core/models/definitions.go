package models

// DefinitionsDTO is the full broker configuration snapshot used for export/import.
type DefinitionsDTO struct {
	OtterMQVersion string                 `json:"ottermq_version"`
	VHosts         []VHostDefinition      `json:"vhosts"`
	Users          []UserDefinition       `json:"users"`
	Permissions    []PermissionDefinition `json:"permissions"`
	Exchanges      []ExchangeDefinition   `json:"exchanges"`
	Queues         []QueueDefinition      `json:"queues"`
	Bindings       []BindingDefinition    `json:"bindings"`
}

type VHostDefinition struct {
	Name string `json:"name"`
}

// UserDefinition holds user credentials as stored in the database.
// PasswordHash is the bcrypt hash — never the plaintext password.
type UserDefinition struct {
	Name         string `json:"name"`
	PasswordHash string `json:"password_hash"`
	Tags         string `json:"tags"` // role name, e.g. "administrator"
}

type PermissionDefinition struct {
	User  string `json:"user"`
	VHost string `json:"vhost"`
}

type ExchangeDefinition struct {
	Name       string         `json:"name"`
	VHost      string         `json:"vhost"`
	Type       string         `json:"type"`
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Internal   bool           `json:"internal"`
	Arguments  map[string]any `json:"arguments,omitempty"`
}

type QueueDefinition struct {
	Name       string         `json:"name"`
	VHost      string         `json:"vhost"`
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Arguments  map[string]any `json:"arguments,omitempty"`
}

type BindingDefinition struct {
	Source          string         `json:"source"`
	VHost           string         `json:"vhost"`
	Destination     string         `json:"destination"`
	DestinationType string         `json:"destination_type"`
	RoutingKey      string         `json:"routing_key"`
	Arguments       map[string]any `json:"arguments,omitempty"`
}

// DefinitionsImportResponse reports the outcome of an import operation.
type DefinitionsImportResponse struct {
	VHosts      int `json:"vhosts_created"`
	Users       int `json:"users_created"`
	Permissions int `json:"permissions_granted"`
	Exchanges   int `json:"exchanges_created"`
	Queues      int `json:"queues_created"`
	Bindings    int `json:"bindings_created"`
}
