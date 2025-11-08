package vhost

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/rs/zerolog/log"
)

type ConsumerKey struct {
	Channel uint16
	Tag     string
}

// ConnectionChannelKey uniquely identifies a channel within a connection
type ConnectionChannelKey struct {
	Connection net.Conn
	Channel    uint16
}

type Consumer struct {
	Tag           string
	Channel       uint16
	QueueName     string
	Connection    net.Conn
	Active        bool
	PrefetchCount uint16 // 0 means unlimited
	Props         *ConsumerProperties
}

type ConsumerProperties struct {
	NoAck     bool           `json:"no_ack"`
	Exclusive bool           `json:"exclusive"`
	Arguments map[string]any `json:"arguments"`
}

func NewConsumer(conn net.Conn, channel uint16, queueName, consumerTag string, props *ConsumerProperties) *Consumer {
	return &Consumer{
		Tag:        consumerTag,
		Channel:    channel,
		QueueName:  queueName,
		Connection: conn,
		Props:      props,
	}
}

func (vh *VHost) RegisterConsumer(consumer *Consumer) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	// Apply consumer prefetch if it was set via QoS(global = false)
	channelKey := ConnectionChannelKey{consumer.Connection, consumer.Channel}
	if state, exists := vh.ChannelDeliveries[channelKey]; exists {
		if !state.PrefetchGlobal && state.NextPrefetchCount > 0 {
			consumer.PrefetchCount = state.NextPrefetchCount
		}
	}

	// Check if queue exists
	_, ok := vh.Queues[consumer.QueueName]
	if !ok {
		// TODO: return a channel exception error - 404
		return fmt.Errorf("queue %s does not exist", consumer.QueueName)
	}

	var key ConsumerKey
	if consumer.Tag == "" {
		const maxRetries = 1000
		var retries int
		for {
			consumer.Tag = generateConsumerTag()
			key = ConsumerKey{consumer.Channel, consumer.Tag}
			if _, exists := vh.Consumers[key]; !exists {
				break
			}
			retries++
			if retries >= maxRetries {
				return fmt.Errorf("failed to generate unique consumer tag after %d attempts", maxRetries)
			}
		}
	} else {
		key = ConsumerKey{consumer.Channel, consumer.Tag}
		// Check for duplicates
		if _, exists := vh.Consumers[key]; exists {
			// TODO: return a channel exception error
			return fmt.Errorf("consumer with tag %s already exists on channel %d", key.Tag, key.Channel)
		}
	}

	// Check if exclusive consumer already exists for this queue
	// or if trying to add exclusive when others exist
	if existingConsumers, exists := vh.ConsumersByQueue[consumer.QueueName]; exists {
		for _, c := range existingConsumers {
			if c.Props.Exclusive {
				// TODO: return a channel exception error - 405
				return fmt.Errorf("exclusive consumer already exists for queue %s", consumer.QueueName)
			}
		}
		if consumer.Props.Exclusive && len(existingConsumers) > 0 {
			// TODO: return a channel exception error - 405
			return fmt.Errorf("cannot add exclusive consumer when other consumers exist for queue %s", consumer.QueueName)
		}
	}

	// Activate consumer and register
	consumer.Active = true
	vh.Consumers[key] = consumer

	// Index for delivery
	// verify if map entry exists
	if _, exists := vh.ConsumersByQueue[consumer.QueueName]; !exists {
		vh.ConsumersByQueue[consumer.QueueName] = []*Consumer{}
	}
	vh.ConsumersByQueue[consumer.QueueName] = append(
		vh.ConsumersByQueue[consumer.QueueName],
		consumer,
	)
	// Index for channel (connection-scoped)
	if _, exists := vh.ConsumersByChannel[channelKey]; !exists {
		vh.ConsumersByChannel[channelKey] = []*Consumer{}
	}
	vh.ConsumersByChannel[channelKey] = append(
		vh.ConsumersByChannel[channelKey],
		consumer,
	)

	// start delivery routine if not already running
	queue := vh.Queues[consumer.QueueName]
	if len(vh.ConsumersByQueue[queue.Name]) == 1 {
		go queue.startDeliveryLoop(vh)
	}

	return nil
}

func (vh *VHost) CancelConsumer(channel uint16, tag string) error {
	key := ConsumerKey{channel, tag}
	vh.mu.Lock()
	defer vh.mu.Unlock()
	consumer, exists := vh.Consumers[key]
	if !exists {
		return fmt.Errorf("consumer with tag %s on channel %d does not exist", tag, channel)
	}

	consumer.Active = false
	delete(vh.Consumers, key)

	// Remove from ConsumersByQueue
	consumersForQueue := vh.ConsumersByQueue[consumer.QueueName]
	for i, c := range consumersForQueue {
		if c.Tag == tag && c.Channel == channel {
			vh.ConsumersByQueue[consumer.QueueName] = append(consumersForQueue[:i], consumersForQueue[i+1:]...)
			break
		}
	}
	if len(vh.ConsumersByQueue[consumer.QueueName]) == 0 {
		queue := vh.Queues[consumer.QueueName]
		queue.stopDeliveryLoop()
		// verify if the queue can be auto-deleted
		if deleted, err := vh.checkAutoDeleteQueueUnlocked(queue.Name); err != nil {
			log.Printf("Failed to check auto-delete queue: %v", err)
		} else if deleted {
			log.Printf("Queue %s was auto-deleted", queue.Name)
		}
	}

	// Remove from ConsumersByChannel (connection-scoped)
	channelKey := ConnectionChannelKey{consumer.Connection, consumer.Channel}
	consumersForChannel := vh.ConsumersByChannel[channelKey]
	for i, c := range consumersForChannel {
		if c.Tag == tag {
			vh.ConsumersByChannel[channelKey] = append(consumersForChannel[:i], consumersForChannel[i+1:]...)
			break
		}
	}

	return nil
}

// checkAutoDeleteQueueUnlocked checks if a queue is auto-delete and has no consumers, and deletes it if so.
func (vh *VHost) checkAutoDeleteQueueUnlocked(name string) (bool, error) {
	queue, exists := vh.Queues[name]
	if !exists {
		return false, fmt.Errorf("queue %s not found", name)
	}
	if !queue.Props.AutoDelete {
		return false, nil
	}

	if len(vh.ConsumersByQueue[name]) > 0 {
		return false, nil
	}

	err := vh.deleteQueuebyNameUnlocked(name)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (vh *VHost) CleanupChannel(connection net.Conn, channel uint16) {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	channelKey := ConnectionChannelKey{connection, channel}
	// Get copy of consumers to avoid modification during iteration
	_ = cancelAllConsumers(vh, channelKey)

	if state := vh.ChannelDeliveries[channelKey]; state != nil {
		state.mu.Lock()
		// copy records to avoid modification during iteration
		records := make([]*DeliveryRecord, 0, len(state.Unacked))
		for _, record := range state.Unacked {
			records = append(records, record)
		}
		state.mu.Unlock()

		// Requeue unacknowledged messages
		for _, record := range records {
			queue, exists := vh.Queues[record.QueueName]
			if exists {
				queue.Push(record.Message)
			}
		}
		delete(vh.ChannelDeliveries, channelKey)
	}

	// Clean up transaction state (already holding vh.mu, so access map directly)
	if txState, exists := vh.ChannelTransactions[channelKey]; exists {
		txState.Lock()
		// Discard buffered publishes and acks (implicit rollback)
		txState.BufferedPublishes = []BufferedPublish{}
		txState.BufferedAcks = []BufferedAck{}
		txState.InTransaction = false
		txState.Unlock()
		delete(vh.ChannelTransactions, channelKey)
	}
}

func cancelAllConsumers(vh *VHost, channelKey ConnectionChannelKey) bool {
	consumersForChannel, exists := vh.ConsumersByChannel[channelKey]
	if !exists {
		return true // No consumers on this channel
	}

	consumersCopy := make([]*Consumer, len(consumersForChannel))
	copy(consumersCopy, consumersForChannel)

	// Cancel each consumer (this will modify the maps)
	for _, c := range consumersCopy {
		// Unlock to call CancelConsumer (which also locks)
		vh.mu.Unlock()
		err := vh.CancelConsumer(c.Channel, c.Tag)
		vh.mu.Lock()
		if err != nil {
			log.Error().Str("consumer", c.Tag).Msg("Error cancelling consumer")
		}

	}
	delete(vh.ConsumersByChannel, channelKey)
	return false
}

func (vh *VHost) CleanupConnection(connection net.Conn) {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	// Find all consumers for this connection
	var consumersToRemove []*Consumer
	for _, consumer := range vh.Consumers {
		if consumer.Connection == connection {
			consumersToRemove = append(consumersToRemove, consumer)
		}
	}

	// Cancel each consumer
	for _, consumer := range consumersToRemove {
		// Unlock to call CancelConsumer (which also locks)
		vh.mu.Unlock()
		err := vh.CancelConsumer(consumer.Channel, consumer.Tag)
		vh.mu.Lock()
		if err != nil {
			log.Error().Str("consumer", consumer.Tag).Msg("Error cancelling consumer")
		}
	}

	// Clean up owned queues
	// it is expected that some (if not all) exclusive queues were already deleted
	// during consumer cancellation above
	var queuesToDelete []string
	for queueName, queue := range vh.Queues {
		if queue.OwnerConn == connection {
			if queue.Props.Exclusive {
				queuesToDelete = append(queuesToDelete, queueName)
			} else {
				queue.OwnerConn = nil
			}
		}
	}
	for _, queueName := range queuesToDelete {
		if err := vh.deleteQueuebyNameUnlocked(queueName); err != nil {
			log.Error().Str("queue", queueName).Msg("Error deleting exclusive queue")
		}
	}
}

func (vh *VHost) GetActiveConsumersForQueue(queueName string) []*Consumer {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	consumers := vh.ConsumersByQueue[queueName]
	if consumers == nil {
		return nil
	}
	// Filter only active consumers
	activeConsumers := make([]*Consumer, 0, len(consumers))
	for _, consumer := range consumers {
		if consumer.Active {
			activeConsumers = append(activeConsumers, consumer)
		}
	}
	return activeConsumers
}

func generatePseudoRandomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func generateConsumerTag() string {
	return "amq.ctag-" + generatePseudoRandomString(12)
}
