package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

func (s *Service) ListQueues(vhostName string) ([]models.QueueDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	// Get statistics from vhost (these methods handle locking internally)
	unackedCounts := vh.GetUnackedMessageCountsAllQueues()
	consumerCounts := vh.GetConsumerCountsAllQueues()
	queues := vh.GetAllQueues()

	// Map to DTOs
	dtos := make([]models.QueueDTO, 0, len(queues))
	for _, queue := range queues {
		dto := s.queueToDTO(vh, queue, unackedCounts[queue.Name], consumerCounts[queue.Name])
		dtos = append(dtos, dto)
	}

	return dtos, nil

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
	unackedCounts := vh.GetUnackedMessageCountsAllQueues()
	consumerCounts := vh.GetConsumersByQueue(queueName)

	dto := s.queueToDTO(vh, queue, unackedCounts[queue.Name], len(consumerCounts))
	return &dto, nil
}

func (s *Service) CreateQueue(req models.CreateQueueRequest) (*models.QueueDTO, error) {
	vhostName := req.VHost
	if vhostName == "" {
		vhostName = "/"
	}

	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	// Not necessary to check for existing queue here;
	// CreateQueue handles that, verifying properties if passive.

	if req.Exclusive {
		// Exclusive queues cannot be created via the management API
		return nil, fmt.Errorf("cannot create exclusive queue via management API")
	}

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
		Passive:    false,
		Durable:    req.Durable,
		AutoDelete: req.AutoDelete,
		Exclusive:  req.Exclusive,
		Arguments:  args,
	}

	// CreateQueue (nil connection = not exclusive via API)
	queue, err := vh.CreateQueue(req.QueueName, &props, nil)
	if err != nil {
		return nil, err
	}

	err = vh.BindToDefaultExchange(queue.Name)
	if err != nil {
		return nil, err
	}

	dto := s.queueToDTO(vh, queue, 0, 0)
	return &dto, nil
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

	count, err := vh.PurgeQueue(queueName, nil)
	return int(count), err
}

func (s *Service) queueToDTO(vh *vhost.VHost, queue *vhost.Queue, unackedCount, consumerCount int) models.QueueDTO {
	messagesReady := queue.Len()
	total := messagesReady + unackedCount

	dto := models.QueueDTO{
		VHost:              vh.Name,
		Name:               queue.Name,
		Messages:           messagesReady,
		MessagesReady:      messagesReady,
		MessagesUnacked:    unackedCount,
		MessagesTotal:      total,
		Consumers:          consumerCount,
		ConsumersActive:    consumerCount,
		Durable:            queue.Props.Durable,
		AutoDelete:         queue.Props.AutoDelete,
		Exclusive:          queue.Props.Exclusive,
		Arguments:          queue.Props.Arguments,
		State:              "running",
		PersistenceEnabled: queue.IsPersistenceEnabled(),
	}

	// Extract DLX configuration
	if dlx, ok := queue.Props.Arguments["x-dead-letter-exchange"].(string); ok {
		dto.DeadLetterExchange = &dlx
	}
	if dlrk, ok := queue.Props.Arguments["x-dead-letter-routing-key"].(string); ok {
		dto.DeadLetterRoutingKey = &dlrk
	}

	// Extract Max Length
	if val, ok := queue.Props.Arguments["x-max-length"]; ok {
		switch v := val.(type) {
		case int32:
			dto.MaxLength = &v
		case int:
			val := int32(v)
			dto.MaxLength = &val
		case int64:
			val := int32(v)
			dto.MaxLength = &val
		case float64:
			val := int32(v)
			dto.MaxLength = &val
		}
	}

	// Extract TTL
	if val, ok := queue.Props.Arguments["x-message-ttl"]; ok {
		switch v := val.(type) {
		case int64:
			dto.MessageTTL = &v
		case int32:
			val := int64(v)
			dto.MessageTTL = &val
		case int:
			val := int64(v)
			dto.MessageTTL = &val
		case float64:
			val := int64(v)
			dto.MessageTTL = &val
		}
	}
	return dto
}
