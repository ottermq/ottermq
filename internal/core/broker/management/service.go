package management

import (
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// Service provides management operations for the broker.
// This replaces the old ManagementAPI interface.

type ManagementService interface {
	/* Queues */

	// ListQueues lists all queues across all vhosts.
	ListQueues() []models.QueueDTO
	// GetQueue retrieves the details of a specific queue within a vhost.
	GetQueue(vhost, queue string) (*models.QueueDTO, error)
	// CreateQueue creates a new queue in the specified vhost.
	CreateQueue(vhost, queue string, req models.CreateQueueRequest) (*models.QueueDTO, error)
	// DeleteQueue deletes a queue from the specified vhost.
	DeleteQueue(vhost, queue string, ifUnused, ifEmpty bool) error
	// PurgeQueue purges all messages from the specified queue within a vhost.
	PurgeQueue(vhost, queue string) (int, error)
	/* Exchanges */

	// ListExchanges lists all exchanges across all vhosts.
	ListExchanges() ([]models.ExchangeDTO, error)
	// GetExchange retrieves the details of a specific exchange within a vhost.
	GetExchange(vhost, exchange string) (*models.ExchangeDTO, error)
	// CreateExchange creates a new exchange in the specified vhost.
	CreateExchange(vhost, exchange string, req models.CreateExchangeRequest) (*models.ExchangeDTO, error)
	// DeleteExchange deletes an exchange from the specified vhost.
	DeleteExchange(vhost, exchange string, ifUnused bool) error

	/* Bindings */

	// ListBindings lists all bindings across all vhosts.
	ListBindings() ([]models.BindingDTO, error)
	// ListVhostBindings lists all bindings within the specified vhost.
	ListVhostBindings(vhost string) ([]models.BindingDTO, error)

	// ListQueueBindings lists all bindings where the specified queue is the destination.
	ListQueueBindings(vhost, queue string) ([]models.BindingDTO, error)
	// ListExchangeBindings lists all bindings where the specified exchange is the source.
	ListExchangeBindings(vhost, exchange string) ([]models.BindingDTO, error)
	// CreateBinding creates a new binding between an exchange and a queue.
	CreateBinding(req models.CreateBindingRequest) (*models.BindingDTO, error)
	// DeleteBinding deletes a binding between an exchange and a queue.
	DeleteBinding(req models.DeleteBindingRequest) error

	/* Consumers */

	// ListConsumers lists all consumers across all vhosts.
	ListConsumers() ([]models.ConsumerDTO, error)
	ListVhostConsumers(vhost string) ([]models.ConsumerDTO, error)
	ListQueueConsumers(vhost, queue string) ([]models.ConsumerDTO, error)

	/* Connections */

	// ListConnections lists all active connections.
	ListConnections() ([]models.ConnectionInfoDTO, error)
	// GetConnection retrieves details of a specific connection.
	GetConnection(name string) (*models.ConnectionInfoDTO, error)
	// CloseConnection closes a specific connection with a given reason.
	CloseConnection(name string, reason string) error

	/* Channels */

	// ListChannels lists all active channels.
	ListChannels(vhost string) ([]models.ChannelDetailDTO, error)
	// ListConnectionChannels lists all channels for a specific connection.
	ListConnectionChannels(connectionName string) ([]models.ChannelDetailDTO, error)
	// GetChannel retrieves details of a specific channel within a connection.
	GetChannel(connectionName string, channelNumber uint16) (*models.ChannelDetailDTO, error)

	// Messages

	// PublishMessage publishes a message to the specified exchange within a vhost.
	PublishMessage(vhost, exchange string, req models.PublishMessageRequest) error
	// GetMessages retrieves messages from the specified queue within a vhost.
	GetMessages(vhost, queue string, count int, ackMode models.AckType) ([]models.MessageDTO, error)

	// VHosts

	// ListVHosts lists all virtual hosts.
	ListVHosts() ([]models.VHostDTO, error)
	// GetVHost retrieves details of a specific virtual host.
	GetVHost(name string) (*models.VHostDTO, error)

	/* Overview/Stats */

	// GetOverview retrieves overall broker statistics and information.
	GetOverview() (*models.OverviewDTO, error)
	// GetBrokerInfo retrieves detailed broker information.
	GetBrokerInfo() models.OverviewBrokerDetails

	// Metrics related methods
	// GetOverviewCharts retrieves time-series data for overview charts
	GetOverviewCharts() (*models.OverviewChartsDTO, error)
}

type Service struct {
	broker BrokerProvider
}

func NewService(b BrokerProvider) *Service {
	return &Service{broker: b}
}
