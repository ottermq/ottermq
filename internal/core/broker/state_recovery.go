package broker

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/pkg/persistence"
	"github.com/rs/zerolog/log"
)

// RecoverBrokerState loads vhosts, exchanges, queues, bindings, and messages from disk
func RecoverBrokerState(b *Broker) error {
	vhostsDir := "data/vhosts"
	entries, err := os.ReadDir(vhostsDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		vhostName := entry.Name()
		options := vhost.VHostOptions{
			QueueBufferSize: b.config.QueueBufferSize,
			Persistence:     b.persist,
		}
		v := vhost.NewVhost(vhostName, options)
		// Recover exchanges
		exchangesDir := filepath.Join(vhostsDir, vhostName, "exchanges")
		exFiles, _ := os.ReadDir(exchangesDir)
		for _, exFile := range exFiles {
			exName := exFile.Name()
			exName = strings.TrimSuffix(exName, ".json")
			// persistedEx, err := b.persist.LoadExchange(vhostName, exName)
			exchangeType, props, err := b.persist.LoadExchangeMetadata(vhostName, exName)

			if err == nil {
				bindings, err := b.persist.LoadExchangeBindings(vhostName, exName)
				if err != nil {
					log.Error().Err(err).Str("exchange", exName).Msg("Failed to load exchange bindings")
					bindings = []persistence.BindingData{}
				}

				// Create exchange in vhost using persistedEx
				v.RecoverExchange(exName, exchangeType, props, bindings)
			}
		}
		queuesDir := filepath.Join(vhostsDir, vhostName, "queues")
		qFiles, _ := os.ReadDir(queuesDir)
		for _, qFile := range qFiles {
			qName := qFile.Name()
			qName = strings.TrimSuffix(qName, ".json")
			props, err := b.persist.LoadQueueMetadata(vhostName, qName)
			if err == nil {
				// Create queue in vhost using persistedQ
				if err := v.RecoverQueue(qName, &props); err != nil {
					log.Error().Err(err).Str("queue", qName).Msg("Failed to recover queue")
				}
			}
		}
		b.VHosts[vhostName] = v
	}
	return nil
}
