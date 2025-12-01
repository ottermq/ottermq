package vhost

import (
	"context"
	"sync"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/andrelcunha/ottermq/internal/persistdb"
	"github.com/andrelcunha/ottermq/pkg/persistence"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	// BASIC_GET_SENTINEL is used as the ConsumerTag for deliveries made via Basic.Get
	// (synchronous pull mode) which don't have an associated consumer. This allows
	// us to track these deliveries separately in the dual-index UnackedByConsumer map.
	BASIC_GET_SENTINEL = "__basic_get__"
)

type VHost struct {
	Name               string                     `json:"name"`
	Id                 string                     `json:"id"`
	Exchanges          map[string]*Exchange       `json:"exchanges"`
	Queues             map[string]*Queue          `json:"queues"`
	Users              map[string]*persistdb.User `json:"users"`
	mu                 sync.Mutex                 `json:"-"`
	queueBufferSize    int                        `json:"-"`
	persist            persistence.Persistence
	Consumers          map[ConsumerKey]*Consumer            `json:"consumers"`          // <- Primary registry
	ConsumersByQueue   map[string][]*Consumer               `json:"consumers_by_queue"` // <- Delivery Index
	ConsumersByChannel map[ConnectionChannelKey][]*Consumer // Index consumers by connection+channel
	activeDeliveries   map[string]context.CancelFunc        // queueName -> cancelFunc
	framer             amqp.Framer
	ChannelDeliveries  map[ConnectionChannelKey]*ChannelDeliveryState

	// redeliveredMessages marks message IDs that must be delivered with
	// Basic.Deliver.redelivered = true on their next successful delivery.
	// We set this when we requeue previously delivered messages (e.g., Basic.Recover(requeue),
	// Basic.Nack/Reject with requeue, or after a delivery failure). Because requeue puts the
	// message back into the queue (losing per-channel unacked metadata), we need this
	// out-of-band memory to preserve AMQP redelivery semantics. The flag is cleared on the
	// next successful delivery.
	redeliveredMessages map[string]struct{} // message ID -> mark
	redeliveredMu       sync.Mutex

	// Transactions per channel
	ChannelTransactions map[ConnectionChannelKey]*ChannelTransactionState

	// Dead letter handler
	DeadLetterer       DeadLetterer
	TTLManager         TTLManager
	QueueLengthLimiter QueueLengthLimiter

	ActiveExtensions map[string]bool

	// injected by the broker to allow consumers to send frames
	frameSender FrameSender
}

type FrameSender interface {
	SendFrame(connID ConnectionID, channel uint16, frame []byte) error
}

type VHostOptions struct {
	QueueBufferSize int
	Persistence     persistence.Persistence
	EnableDLX       bool
	EnableTTL       bool
	EnableQLL       bool
}

func NewVhost(vhostName string, options VHostOptions) *VHost {
	id := uuid.New().String()
	vh := &VHost{
		Name:                vhostName,
		Id:                  id,
		Exchanges:           make(map[string]*Exchange),
		Queues:              make(map[string]*Queue),
		Users:               make(map[string]*persistdb.User),
		queueBufferSize:     options.QueueBufferSize,
		persist:             options.Persistence,
		Consumers:           make(map[ConsumerKey]*Consumer),
		ConsumersByQueue:    make(map[string][]*Consumer),
		ConsumersByChannel:  make(map[ConnectionChannelKey][]*Consumer),
		activeDeliveries:    make(map[string]context.CancelFunc),
		ChannelDeliveries:   make(map[ConnectionChannelKey]*ChannelDeliveryState),
		redeliveredMessages: make(map[string]struct{}),
		ChannelTransactions: make(map[ConnectionChannelKey]*ChannelTransactionState),
		ActiveExtensions:    make(map[string]bool),
	}
	vh.createMandatoryStructure()
	// Load persisted state
	if options.Persistence != nil {
		vh.loadPersistedState()
	}
	vh.setupExtensions(options)

	return vh
}

func (vh *VHost) SetFrameSender(sender FrameSender) {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	vh.frameSender = sender
}

func (vh *VHost) setupExtensions(options VHostOptions) {
	if options.EnableDLX {
		vh.ActiveExtensions["dlx"] = true
		vh.DeadLetterer = &DeadLetter{vh: vh}
	} else {
		vh.DeadLetterer = &NoOpDeadLetterer{}
	}
	if options.EnableTTL {
		vh.ActiveExtensions["ttl"] = true
		vh.TTLManager = &DefaultTTLManager{vh: vh}
	} else {
		vh.TTLManager = &NoOpTTLManager{}
	}
	if options.EnableQLL {
		vh.ActiveExtensions["qll"] = true
		vh.QueueLengthLimiter = &DefaultQueueLengthLimiter{vh: vh}
	} else {
		vh.QueueLengthLimiter = &NoOpQueueLengthLimiter{}
	}
}

func (vh *VHost) SetFramer(framer amqp.Framer) {
	vh.framer = framer
}

func (vh *VHost) createMandatoryStructure() {
	vh.createMandatoryExchanges()
}

// GetUnackedMessageCountsAllQueues returns a map of queue names to their respective counts of unacknowledged messages.
func (vh *VHost) GetUnackedMessageCountsAllQueues() map[string]int {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	counts := make(map[string]int)
	for _, channelState := range vh.ChannelDeliveries {
		channelState.mu.Lock()
		for _, record := range channelState.UnackedByTag {
			counts[record.QueueName]++
		}
		channelState.mu.Unlock()
	}
	return counts
}

// GetUnackedMessageCountByChannel returns the count of unacknowledged messages for a specific connection and channel.
func (vh *VHost) GetUnackedMessageCountByChannel(connID ConnectionID, channel uint16) int {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	key := ConnectionChannelKey{connID, channel}
	channelState := vh.ChannelDeliveries[key]
	if channelState == nil {
		return 0
	}

	channelState.mu.Lock()
	count := len(channelState.UnackedByTag)
	channelState.mu.Unlock()

	return count
}

// GetConsumerCountsAllQueues returns a map of queue names to the number of active consumers.
func (vh *VHost) GetConsumerCountsAllQueues() map[string]int {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	counts := make(map[string]int)
	for queueName, consumers := range vh.ConsumersByQueue {
		counts[queueName] = len(consumers)
	}
	return counts
}

// GetConsumersByQueue returns a slice of consumers for the specified queue.
func (vh *VHost) GetConsumersByQueue(queueName string) []*Consumer {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	return vh.ConsumersByQueue[queueName]
}

// GetExchangeUniqueNames returns a map of unique exchange names across all vhosts.
// Necessary to avoid duplicates when listing exchanges (e.g., default exchange "" and amqp.default).
// It lists the default exchange as "(AMQP default)", which is more client-friendly than "" (empty string).

func (vh *VHost) GetExchangeUniqueNames() map[string]bool {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	exchangeNames := make(map[string]bool)
	for _, exchange := range vh.Exchanges {
		if exchange.Name != EMPTY_EXCHANGE {
			exchangeNames[exchange.Name] = true
		}
	}

	return exchangeNames
}

// GetAllExchanges returns a slice of all (deduplicated) exchanges in this vhost.
func (vh *VHost) GetAllExchanges() []*Exchange {
	exchangeNames := vh.GetExchangeUniqueNames()
	vh.mu.Lock()
	defer vh.mu.Unlock()
	exchanges := make([]*Exchange, 0, len(exchangeNames))
	for name := range exchangeNames {
		exchanges = append(exchanges, vh.Exchanges[name])
	}
	return exchanges
}

// GetExchange retrieves an exchange by name.
func (vh *VHost) GetExchange(exchangeName string) *Exchange {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	// get actual exchange name in case an alias was used
	actualName := exchangeName
	if exchangeName == DEFAULT_EXCHANGE_ALIAS || exchangeName == EMPTY_EXCHANGE {
		actualName = DEFAULT_EXCHANGE
	}
	return vh.Exchanges[actualName]
}

// GetAllQueues returns a copy of all queues in this vhost.
func (vh *VHost) GetAllQueues() []*Queue {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	queues := make([]*Queue, 0, len(vh.Queues))
	for _, queue := range vh.Queues {
		queues = append(queues, queue)
	}
	return queues
}

// GetQueue retrieves a queue by name.
func (vh *VHost) GetQueue(queueName string) *Queue {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	return vh.Queues[queueName]
}

func (vh *VHost) loadPersistedState() {
	if vh.persist == nil {
		return
	}
	// Load queues
	queues, err := vh.persist.LoadAllQueues(vh.Name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load queues from persistence")
	}
	// create a map of skipped queues due to exclusivity
	skippedQueues := make(map[string]bool)
	for _, queue := range queues {
		// if queue is exclusive, it should not be restored
		if queue.Properties.Exclusive {
			skippedQueues[queue.Name] = true
			continue
		}
		vh.Queues[queue.Name] = NewQueue(queue.Name, vh.queueBufferSize, vh)
		vh.Queues[queue.Name].Props = &QueueProperties{
			Durable:    queue.Properties.Durable,
			AutoDelete: queue.Properties.AutoDelete,
			Exclusive:  queue.Properties.Exclusive,
			Arguments:  queue.Properties.Arguments,
		}
		// Load messages into the queue
		for _, msgData := range queue.Messages {
			msg := FromPersistence(msgData)
			vh.Queues[queue.Name].Push(msg)
		}
	}

	// Load exchanges
	exchanges, err := vh.persist.LoadAllExchanges(vh.Name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load exchanges from persistence")
		return
	}
	for _, exchange := range exchanges {
		typ := ExchangeType(exchange.Type)
		props := &ExchangeProperties{
			Durable:    exchange.Properties.Durable,
			AutoDelete: exchange.Properties.AutoDelete,
			Arguments:  exchange.Properties.Arguments,
		}
		vh.Exchanges[exchange.Name] = NewExchange(exchange.Name, typ, props)
		for _, bindingData := range exchange.Bindings {
			// Skip bindings to skipped exclusive queues
			if skippedQueues[bindingData.QueueName] {
				continue
			}
			err := vh.BindQueue(exchange.Name, bindingData.QueueName, bindingData.RoutingKey, exchange.Properties.Arguments, INTERNAL_CONN_ID)
			if err != nil {
				log.Error().Err(err).Str("exchange", exchange.Name).Msg("Error binding queue")
			}
		}
	}
}

func (vh *VHost) ListConsumers(cb func(queueName string, consumer Consumer, dtos []models.ConsumerDTO)) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	// Iterate through all queues and their consumers
	for queueName, queueConsumers := range vh.ConsumersByQueue {
		for _, consumer := range queueConsumers {
			cb(queueName, *consumer, nil)
		}
	}
	return nil
}

type NoConsumersError interface {
	error
}

func (vh *VHost) ListQueueConsumers(queueName string, cb func(consumer Consumer, dtos []any)) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	queueConsumers, err := vh.getConsumersByQueue(queueName)
	if err != nil {
		return err
	}
	if len(queueConsumers) == 0 {
		return nil // No consumers
	}

	consumers := make([]any, 0, len(queueConsumers))
	for _, consumer := range queueConsumers {
		cb(*consumer, consumers)
	}
	return nil

}

func (vh *VHost) getConsumersByQueue(queueName string) ([]*Consumer, error) {
	queueConsumers, exists := vh.ConsumersByQueue[queueName]
	if !exists {
		return nil, nil
	}
	return queueConsumers, nil
}
