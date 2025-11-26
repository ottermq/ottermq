package management

import (
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// Service provides management operations for the broker.
// This replaces the old ManagementAPI interface.

type ManagementService interface {
	// Queues
	ListQueues(vhost string) ([]models.QueueDTO, error)
	GetQueue(vhost, name string) (*models.QueueDTO, error)
	CreateQueue(req models.CreateQueueRequest) (*models.QueueDTO, error)
	DeleteQueue(vhost, name string, ifUnused, ifEmpty bool) error
	PurgeQueue(vhost, name string) (int, error)

	// Exchanges
	ListExchanges(vhost string) ([]models.ExchangeDTO, error)
	GetExchange(vhost, name string) (*models.ExchangeDTO, error)
	CreateExchange(req models.CreateExchangeRequest) (*models.ExchangeDTO, error)
	DeleteExchange(vhost, name string, ifUnused bool) error

	// Bindings
	ListBindings(vhost string) ([]models.BindingDTO, error)
	ListQueuesBindings(vhost, queue string) ([]models.BindingDTO, error)
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
}

type Service struct {
	broker BrokerProvider
}

func NewService(b BrokerProvider) *Service {
	return &Service{broker: b}
}
