package management

import (
	"github.com/ottermq/ottermq/internal/core/amqp"
	"github.com/ottermq/ottermq/internal/core/broker/vhost"
	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/ottermq/ottermq/pkg/metrics"
)

// BrokerProvider defines the minimal interface that management operations need from the broker
type BrokerProvider interface {
	GetVHost(vhostName string) *vhost.VHost
	ListVHosts() []*vhost.VHost
	ListVhostDetails() ([]models.VHostDTO, error)
	CreateVhostDto(vh *vhost.VHost) (models.VHostDTO, error)
	CreateVHost(name string) error
	DeleteVHost(name string) error
	ListConnections() []amqp.ConnectionInfo
	ListChannels(vhost string) ([]models.ChannelInfo, error)
	ListConnectionChannels(connectionName string) ([]models.ChannelInfo, error)
	CreateChannelInfo(connID vhost.ConnectionID, channelNum uint16, vh *vhost.VHost) (models.ChannelInfo, error)
	GetConnectionByName(name string) (*amqp.ConnectionInfo, error)
	GetOverviewConnStats() models.OverviewConnectionStats
	GetBrokerOverviewConfig() models.BrokerConfigOverview
	GetBrokerOverviewDetails() models.OverviewBrokerDetails
	GetOverviewNodeDetails() models.OverviewNodeDetails
	GetObjectTotalsOverview() models.OverviewObjectTotals
	CloseConnection(name string, reason string) error

	// Metrics related methods
	GetCollector() metrics.MetricsCollector
}
