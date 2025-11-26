package management

import (
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

// fakeBroker is a lightweight test implementation of BrokerProvider avoiding circular imports.
type fakeBroker struct {
	vhosts map[string]*vhost.VHost
}

func (fb *fakeBroker) GetVHost(name string) *vhost.VHost { return fb.vhosts[name] }
func (fb *fakeBroker) ListVHosts() []*vhost.VHost {
	out := make([]*vhost.VHost, 0, len(fb.vhosts))
	for _, vh := range fb.vhosts {
		out = append(out, vh)
	}
	return out
}

// setupTestBroker creates a single default vhost and returns a BrokerProvider.
func setupTestBroker(t *testing.T) BrokerProvider {
	t.Helper()
	// Provide queue buffer size for tests; enable minimal extensions disabled by default.
	options := vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
		EnableDLX:       false,
		EnableTTL:       false,
		EnableQLL:       false,
	}
	vh := vhost.NewVhost("/", options)
	// Some tests may rely on time-based logic later; ensure deterministic start (placeholder).
	_ = time.Now()
	return &fakeBroker{vhosts: map[string]*vhost.VHost{"/": vh}}
}
