package management

import (
	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// BrokerProvider defines the minimal interface that management operations need from the broker
type BrokerProvider interface {
	GetVHost(vhostName string) *vhost.VHost
	ListVHosts() []*vhost.VHost
	ListConnections() []amqp.ConnectionInfo
	ListChannels() ([]models.ChannelInfo, error)
	ListConnectionChannels(connectionName string) ([]models.ChannelInfo, error)
	CreateChannelInfo(connID vhost.ConnectionID, channelNum uint16, vh *vhost.VHost) (models.ChannelInfo, error)
	GetConnectionByName(name string) (*amqp.ConnectionInfo, error)
}
