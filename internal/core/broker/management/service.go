package management

import (
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// Service provides management operations for the broker.
// This replaces the old ManagementAPI interface.

type ManagementService interface {
	// Queues
	// ListQueues lists all queues across all vhosts.
	ListQueues() []models.QueueDTO
	// GetQueue retrieves the details of a specific queue within a vhost.
	GetQueue(vhost, queue string) (*models.QueueDTO, error)
	// CreateQueue creates a new queue in the specified vhost.
	CreateQueue(vhost, queue string, req models.CreateQueueRequest) (*models.QueueDTO, error)
	DeleteQueue(vhost, queue string, ifUnused, ifEmpty bool) error
	PurgeQueue(vhost, queue string) (int, error)
	// Exchanges
	// ListExchanges lists all exchanges across all vhosts.
	ListExchanges() ([]models.ExchangeDTO, error)
	// GetExchange retrieves the details of a specific exchange within a vhost.
	GetExchange(vhost, exchange string) (*models.ExchangeDTO, error)
	// CreateExchange creates a new exchange in the specified vhost.
	CreateExchange(vhost, exchange string, req models.CreateExchangeRequest) (*models.ExchangeDTO, error)
	DeleteExchange(vhost, exchange string, ifUnused bool) error

	// Bindings
	ListBindings(vhost string) ([]models.BindingDTO, error)
	ListQueueBindings(vhost, queue string) ([]models.BindingDTO, error)
	ListExchangeBindings(vhost, exchange string) ([]models.BindingDTO, error)
	CreateBinding(req models.CreateBindingRequest) (*models.BindingDTO, error)
	DeleteBinding(req models.DeleteBindingRequest) error

	// Consumers
	ListConsumers(vhost string) ([]models.ConsumerDTO, error)
	ListQueueConsumers(vhost, queue string) ([]models.ConsumerDTO, error)

	// Connections
	ListConnections() ([]models.ConnectionInfoDTO, error)
	GetConnection(name string) (*models.ConnectionInfoDTO, error)
	CloseConnection(name string, reason string) error

	// Channels
	ListChannels() ([]models.ChannelDTO, error)
	ListConnectionChannels(connectionName string) ([]models.ChannelDTO, error)
	GetChannel(connectionName string, channelNumber uint16) (*models.ChannelDTO, error)

	// Messages
	PublishMessage(req models.PublishMessageRequest) error
	GetMessages(vhost, queue string, count int, ackMode models.AckType) ([]models.MessageDTO, error)

	// VHosts
	ListVHosts() ([]models.VHostDTO, error)
	GetVHost(name string) (*models.VHostDTO, error)

	// Overview/Stats
	GetOverview() (*models.OverviewDTO, error)
	GetBrokerInfo() models.OverviewBrokerDetails
}

type Service struct {
	broker BrokerProvider
}

func NewService(b BrokerProvider) *Service {
	return &Service{broker: b}
}
