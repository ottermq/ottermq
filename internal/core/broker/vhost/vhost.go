package vhost

import (
	"context"
	"net"
	"sync"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/persistdb"
	"github.com/andrelcunha/ottermq/pkg/persistence"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
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
}

type VHostOptions struct {
	QueueBufferSize int
	Persistence     persistence.Persistence
	EnableDLX       bool
	EnableTTL       bool
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
		for _, record := range channelState.Unacked {
			counts[record.QueueName]++
		}
		channelState.mu.Unlock()
	}
	return counts
}

// GetUnackedMessageCountByChannel returns the count of unacknowledged messages for a specific connection and channel.
func (vh *VHost) GetUnackedMessageCountByChannel(conn net.Conn, channel uint16) int {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	key := ConnectionChannelKey{conn, channel}
	channelState := vh.ChannelDeliveries[key]
	if channelState == nil {
		return 0
	}

	channelState.mu.Lock()
	count := len(channelState.Unacked)
	channelState.mu.Unlock()

	return count
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
			err := vh.BindQueue(exchange.Name, bindingData.QueueName, bindingData.RoutingKey, exchange.Properties.Arguments, nil)
			if err != nil {
				log.Error().Err(err).Str("exchange", exchange.Name).Msg("Error binding queue")
			}
		}
	}
}
