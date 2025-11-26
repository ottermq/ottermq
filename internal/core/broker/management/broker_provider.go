package management

import (
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
)

// BrokerProvider defines the minimal interface that management operations need from the broker
type BrokerProvider interface {
	GetVHost(vhostName string) *vhost.VHost
	ListVHosts() []*vhost.VHost
}
