package vhost

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
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

func (vh *VHost) RegisterConsumer(consumer *Consumer) (string, error) {
	vh.mu.Lock()

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
		vh.mu.Unlock()
		return "", errors.NewChannelError(
			fmt.Sprintf("no queue '%s' in vhost '%s'", consumer.QueueName, vh.Name),
			uint16(amqp.NOT_FOUND),
			uint16(amqp.BASIC),
			uint16(amqp.BASIC_CONSUME),
		)
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
				vh.mu.Unlock()
				// return fmt.Errorf("failed to generate unique consumer tag after %d attempts", maxRetries)
				return "", errors.NewChannelError(
					"failed to generate unique consumer tag",
					uint16(amqp.INTERNAL_ERROR),
					uint16(amqp.BASIC),
					uint16(amqp.BASIC_CONSUME),
				)
			}
		}
	} else {
		key = ConsumerKey{consumer.Channel, consumer.Tag}
		// Check for duplicates
		if _, exists := vh.Consumers[key]; exists {
			vh.mu.Unlock()
			return consumer.Tag, errors.NewChannelError(
				fmt.Sprintf("consumer tag '%s' already exists on channel %d", key.Tag, key.Channel),
				uint16(amqp.PRECONDITION_FAILED),
				uint16(amqp.BASIC),
				uint16(amqp.BASIC_CONSUME),
			)
		}
	}

	// Check if exclusive consumer already exists for this queue
	// or if trying to add exclusive when others exist
	if existingConsumers, exists := vh.ConsumersByQueue[consumer.QueueName]; exists {
		for _, c := range existingConsumers {
			if c.Props.Exclusive {
				vh.mu.Unlock()
				return "", errors.NewChannelError(
					fmt.Sprintf("exclusive consumer already exists for queue %s", consumer.QueueName),
					uint16(amqp.PRECONDITION_FAILED),
					uint16(amqp.BASIC),
					uint16(amqp.BASIC_CONSUME),
				)
			}
		}
		if consumer.Props.Exclusive && len(existingConsumers) > 0 {
			vh.mu.Unlock()
			// TODO: return a channel exception error - 405
			// return fmt.Errorf("cannot add exclusive consumer when other consumers exist for queue %s", consumer.QueueName)
			return consumer.Tag, errors.NewChannelError(
				fmt.Sprintf("cannot obtain exclusive access to queue '%s' because it is already in use", consumer.QueueName),
				uint16(amqp.RESOURCE_LOCKED),
				uint16(amqp.BASIC),
				uint16(amqp.BASIC_CONSUME),
			)
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

	// Check if we need to start delivery routine
	queue := vh.Queues[consumer.QueueName]
	shouldStartDelivery := len(vh.ConsumersByQueue[queue.Name]) == 1

	// Release the lock BEFORE starting the delivery loop to avoid deadlock
	// The delivery loop may need to acquire locks on both vh and queue
	vh.mu.Unlock()

	// start delivery routine if not already running
	if shouldStartDelivery {
		go queue.startDeliveryLoop(vh)
	}

	return consumer.Tag, nil
}

func (vh *VHost) CancelConsumer(channel uint16, tag string) error {
	key := ConsumerKey{channel, tag}
	vh.mu.Lock()
	defer vh.mu.Unlock()
	consumer, exists := vh.Consumers[key]
	if !exists {
		return errors.NewChannelError(
			fmt.Sprintf("no consumer '%s' on channel %d", tag, channel),
			uint16(amqp.NOT_FOUND),
			uint16(amqp.BASIC),
			uint16(amqp.BASIC_CANCEL),
		)
	}

	consumer.Active = false
	delete(vh.Consumers, key)

	// Requeue unacked messages for this consumer
	channelKey := ConnectionChannelKey{consumer.Connection, consumer.Channel}
	if state := vh.ChannelDeliveries[channelKey]; state != nil {
		vh.requeueUnackedForConsumer(state, consumer.Tag, consumer.QueueName)
	}

	// Remove from ConsumersByQueue
	consumersForQueue := vh.ConsumersByQueue[consumer.QueueName]
	for i, c := range consumersForQueue {
		if c.Tag == tag && c.Channel == channel {
			vh.ConsumersByQueue[consumer.QueueName] = append(consumersForQueue[:i], consumersForQueue[i+1:]...)
			break
		}
	}
	if len(vh.ConsumersByQueue[consumer.QueueName]) == 0 {
		queue, exists := vh.Queues[consumer.QueueName]
		if exists {
			vh.mu.Unlock()
			queue.stopDeliveryLoop()
			vh.mu.Lock()

			// verify if the queue can be auto-deleted
			if deleted, err := vh.checkAutoDeleteQueueUnlocked(queue.Name); err != nil {
				log.Printf("Failed to check auto-delete queue: %v", err)
			} else if deleted {
				log.Printf("Queue %s was auto-deleted", queue.Name)
			}
		}
	}

	// Remove from ConsumersByChannel (connection-scoped)
	channelKey = ConnectionChannelKey{consumer.Connection, consumer.Channel}
	consumersForChannel := vh.ConsumersByChannel[channelKey]
	for i, c := range consumersForChannel {
		if c.Tag == tag {
			vh.ConsumersByChannel[channelKey] = append(consumersForChannel[:i], consumersForChannel[i+1:]...)
			break
		}
	}

	return nil
}

// requeueUnackedForConsumer requeues all unacked messages for a specific consumer.
// This is called when a consumer is canceled per AMQP spec.
// The vh.mu lock MUST be held by caller.
func (vh *VHost) requeueUnackedForConsumer(state *ChannelDeliveryState, consumerTag, queueName string) {
	state.mu.Lock()

	consumerUnacked := state.UnackedByConsumer[consumerTag]
	if consumerUnacked == nil || len(consumerUnacked) == 0 {
		state.mu.Unlock()
		return // No unacked messages for this consumer
	}

	// retrieve from nested index
	delete(state.UnackedByConsumer, consumerTag)

	// Remove from flat index
	for deliveryTag := range consumerUnacked {
		delete(state.UnackedByTag, deliveryTag)
	}

	// Copy recordsToRequeue for processing outside lock
	recordsToRequeue := make([]*DeliveryRecord, 0, len(consumerUnacked))
	for _, record := range consumerUnacked {
		recordsToRequeue = append(recordsToRequeue, record)
	}
	state.mu.Unlock()

	// Requeue messages (outside state lock to avoid deadlock with queue.Push)
	queue, exists := vh.Queues[queueName]
	if !exists {
		log.Warn().
			Str("consumer", consumerTag).
			Str("queue", queueName).
			Int("count", len(recordsToRequeue)).
			Msg("Cannot requeue unacked messages: queue not found")
		return
	}

	log.Debug().
		Str("consumer", consumerTag).
		Str("queue", queueName).
		Int("count", len(recordsToRequeue)).
		Msg("Requeuing unacked messages for canceled consumer")

	for _, record := range recordsToRequeue {
		// Mark as redelivered for next delivery
		vh.markAsRedelivered(record.Message.ID)

		// requeue to original queue
		queue.Push(record.Message)
	}
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
		unackedMessages := fetchUnackedMessages(state)

		state.mu.Unlock()

		// Requeue unacknowledged messages
		for _, record := range unackedMessages {
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

// fetchUnackedMessages retrieves all unacknowledged delivery records from the channel delivery state.
func fetchUnackedMessages(state *ChannelDeliveryState) []*DeliveryRecord {
	// Count total unacked messages
	totalUnacked := 0
	for _, consumeMap := range state.UnackedByConsumer {
		totalUnacked += len(consumeMap)
	}

	records := make([]*DeliveryRecord, 0, totalUnacked)
	for _, consumeMap := range state.UnackedByConsumer {
		for _, record := range consumeMap {
			records = append(records, record)
		}
	}
	return records
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
