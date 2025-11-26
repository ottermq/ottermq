package management

import "github.com/andrelcunha/ottermq/internal/core/models"

func (s *Service) PublishMessage(vhost string, req models.PublishMessageRequest) error {
	panic("not implemented")
}

func (s *Service) GetMessages(vhost, queue string, count int, requeue, ackMode string) ([]models.MessageDTO, error) {
	panic("not implemented")
}
