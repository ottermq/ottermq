package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// ListConsumers lists all consumers.
func (s *Service) ListConsumers() ([]models.ConsumerDTO, error) {
	vhosts := s.broker.ListVHosts()
	var dtos []models.ConsumerDTO
	for _, vh := range vhosts {
		consumers, err := s.newFunction(vh, []models.ConsumerDTO{})
		if err != nil {
			return nil, err
		}
		dtos = append(dtos, consumers...)
	}
	return dtos, nil
}

func (s *Service) ListVhostConsumers(vhostName string) ([]models.ConsumerDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}
	return s.newFunction(vh, []models.ConsumerDTO{})
}

func (s *Service) newFunction(vh *vhost.VHost, consumers []models.ConsumerDTO) ([]models.ConsumerDTO, error) {
	err := vh.ListConsumers(func(queueName string, consumer vhost.Consumer, dtos []models.ConsumerDTO) {
		dto := s.consumerToDTO(vh, queueName, &consumer)
		consumers = append(consumers, dto)
	})

	if err != nil {
		return nil, err
	}

	return consumers, nil
}

// ListQueueConsumers lists all consumers for a specific queue in the specified vhost.
func (s *Service) ListQueueConsumers(vhostName, queueName string) ([]models.ConsumerDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}
	consumers := vh.GetConsumersByQueue(queueName)
	var dtos []models.ConsumerDTO
	for _, consumer := range consumers {
		dto := s.consumerToDTO(vh, queueName, consumer)
		dtos = append(dtos, dto)
	}

	return dtos, nil
}

func (s *Service) consumerToDTO(vh *vhost.VHost, queueName string, consumer *vhost.Consumer) models.ConsumerDTO {
	// Get channel details from connection
	var channelDetails models.ChannelDetailsDTO

	// Find connection for this consumer
	connections := s.broker.ListConnections()
	for _, conn := range connections {
		if conn.VHostName == vh.Name {
			// Match consumer's channel to connection
			// This requires adding channel tracking to ConnectionInfo
			channelDetails = models.ChannelDetailsDTO{
				Number:         consumer.Channel,
				ConnectionName: conn.Client.Conn.RemoteAddr().String(),
				User:           conn.Client.Config.Username,
			}
			break
		}
	}

	return models.ConsumerDTO{
		ConsumerTag:    consumer.Tag,
		QueueName:      queueName,
		ChannelDetails: channelDetails,
		AckRequired:    !consumer.Props.NoAck,
		Exclusive:      consumer.Props.Exclusive,
		PrefetchCount:  int(consumer.PrefetchCount),
		Active:         true, // Consumer exists, so it's active
		Arguments:      consumer.Props.Arguments,
	}
}
