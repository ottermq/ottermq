package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// ListQueues returns a list of all queues across all vhosts.
func (s *Service) ListQueues() []models.QueueDTO {
	// get queues from all vhosts
	var dtos []models.QueueDTO
	vhosts := s.broker.ListVHosts()
	queues := []*vhost.Queue{}

	for _, vh := range vhosts {
		queues = append(queues, vh.GetAllQueues()...)
		// Map to DTOs
		for _, queue := range queues {
			stats := s.fetchQueueStatistics(vh, queue)
			dto := s.queueToDTO(vh, queue, stats)
			dtos = append(dtos, dto)
		}
	}
	return dtos
}

type QueueStats struct {
	Messages      int
	UnackedCount  int
	ConsumerCount int
}

func (*Service) fetchQueueStatistics(vh *vhost.VHost, queue *vhost.Queue) *QueueStats {
	var messages, unackedCount, consumerCount int
	collector := vh.GetCollector()
	if collector != nil && collector.GetQueueSnapshot(queue.Name) != nil {
		snapshot := collector.GetQueueSnapshot(queue.Name)
		messages = int(snapshot.MessageCount)
		unackedCount = int(snapshot.UnackedCount)
		consumerCount = int(snapshot.ConsumerCount)
	} else {
		messages = queue.Len()
		unackedCounts := vh.GetUnackedMessageCountsAllQueues()
		consumerCounts := vh.GetConsumerCountsAllQueues()
		unackedCount = unackedCounts[queue.Name]
		consumerCount = consumerCounts[queue.Name]
	}
	return &QueueStats{
		Messages:      messages,
		UnackedCount:  unackedCount,
		ConsumerCount: consumerCount,
	}
}

func (s *Service) GetQueue(vhostName, queueName string) (*models.QueueDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	queue := vh.GetQueue(queueName)
	if queue == nil {
		return nil, fmt.Errorf("queue '%s' not found in vhost '%s'", queueName, vhostName)
	}

	// Get statistics from vhost (these methods handle locking internally)
	stats := s.fetchQueueStatistics(vh, queue)
	dto := s.queueToDTO(vh, queue, stats)
	return &dto, nil
}

// CreateQueue creates a new queue in the specified vhost.
// TODO: handle idempotent creation (passive + same properties) -- it seems that it is created twice
func (s *Service) CreateQueue(vhostName, queueName string, req models.CreateQueueRequest) (*models.QueueDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	// Not necessary to check for existing queue here;
	// CreateQueue handles that, verifying properties if passive.

	props := createQueueProperties(req)

	// CreateQueue (nil connection = not exclusive via API)
	queue, err := vh.CreateQueue(queueName, &props, vhost.MANAGEMENT_CONNECTION_ID)
	if err != nil {
		return nil, err
	}

	err = vh.BindToDefaultExchange(queue.Name)
	if err != nil {
		return nil, err
	}

	stats := s.fetchQueueStatistics(vh, queue)
	dto := s.queueToDTO(vh, queue, stats)
	return &dto, nil
}

func createQueueProperties(req models.CreateQueueRequest) vhost.QueueProperties {
	// Build arguments from convenience fields
	args := req.Arguments
	if args == nil {
		args = make(map[string]any)
	}

	// Map convenience fields to arguments
	if req.MaxLength != nil {
		args["x-max-length"] = *req.MaxLength
	}
	if req.MessageTTL != nil {
		args["x-message-ttl"] = *req.MessageTTL
	}
	if req.DeadLetterExchange != nil {
		args["x-dead-letter-exchange"] = *req.DeadLetterExchange
	}
	if req.DeadLetterRoutingKey != nil {
		args["x-dead-letter-routing-key"] = *req.DeadLetterRoutingKey
	}

	props := vhost.QueueProperties{
		Passive:    req.Passive,
		Durable:    req.Durable,
		AutoDelete: req.AutoDelete,
		Exclusive:  false,
		Arguments:  args,
	}
	return props
}

func (s *Service) DeleteQueue(vhostName, queueName string, ifUnused, ifEmpty bool) error {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return fmt.Errorf("vhost '%s' not found", vhostName)
	}

	queue := vh.GetQueue(queueName)
	if queue == nil {
		return nil // Idempotent delete
	}

	// Check conditions
	if ifUnused {
		consumerCount := len(vh.ConsumersByQueue[queueName])
		if consumerCount > 0 {
			return fmt.Errorf("queue '%s' has %d consumers, cannot delete (if-unused)", queueName, consumerCount)
		}
	}

	if ifEmpty {
		if queue.Len() > 0 {
			return fmt.Errorf("queue '%s' has %d messages, cannot delete (if-empty)", queueName, queue.Len())
		}
	}

	return vh.DeleteQueuebyName(queueName)

}

func (s *Service) PurgeQueue(vhostName, queueName string) (int, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return 0, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	count, err := vh.PurgeQueue(queueName, vhost.MANAGEMENT_CONNECTION_ID)
	return int(count), err
}

func (s *Service) queueToDTO(vh *vhost.VHost, queue *vhost.Queue, stats *QueueStats) models.QueueDTO {
	total := stats.Messages + stats.UnackedCount

	dto := models.QueueDTO{
		VHost:              vh.Name,
		Name:               queue.Name,
		Messages:           stats.Messages,
		MessagesReady:      stats.Messages,
		MessagesUnacked:    stats.UnackedCount,
		MessagesTotal:      total,
		Consumers:          stats.ConsumerCount,
		Durable:            queue.Props.Durable,
		AutoDelete:         queue.Props.AutoDelete,
		Exclusive:          queue.Props.Exclusive,
		Arguments:          queue.Props.Arguments,
		State:              "running",
		PersistenceEnabled: queue.IsPersistenceEnabled(),
	}
	if queue.Props.Arguments == nil {
		return dto
	}

	// Extract DLX configuration
	if dlx, ok := queue.Props.Arguments["x-dead-letter-exchange"].(string); ok {
		if dlx != "" {
			dto.DeadLetterExchange = &dlx
		}
	}
	if dlrk, ok := queue.Props.Arguments["x-dead-letter-routing-key"].(string); ok {
		if dlrk != "" {
			dto.DeadLetterRoutingKey = &dlrk
		}
	}

	// Extract Max Length
	if val, ok := queue.Props.Arguments["x-max-length"]; ok {
		if val != nil && val != 0 {
			dto.MaxLength = toInt32Pointer(val)
		}
	}

	// Extract TTL
	if val, ok := queue.Props.Arguments["x-message-ttl"]; ok {
		if val != nil && val != 0 {
			dto.MessageTTL = toInt64Pointer(val)
		}
	}

	// Extract Max Priority
	if val, ok := queue.Props.Arguments["x-max-priority"]; ok {
		if val != nil && val != 0 {
			dto.MaxPriority = toUint8Pointer(val)
		}
	}

	return dto
}

func toInt32Pointer(val any) *int32 {
	switch v := val.(type) {
	case int32:
		return &v
	case int:
		val := int32(v)
		return &val
	case int64:
		val := int32(v)
		return &val
	case float64:
		val := int32(v)
		return &val
	}
	return nil
}

func toInt64Pointer(val any) *int64 {
	switch v := val.(type) {
	case int64:
		return &v
	case int32:
		val := int64(v)
		return &val
	case int:
		val := int64(v)
		return &val
	case float64:
		val := int64(v)
		return &val
	}
	return nil
}

func toUint8Pointer(val any) *uint8 {
	switch v := val.(type) {
	case uint8:
		return &v
	case int:
		val := uint8(v)
		return &val
	case int32:
		val := uint8(v)
		return &val
	case int64:
		val := uint8(v)
		return &val
	case float64:
		val := uint8(v)
		return &val
	}
	return nil
}
