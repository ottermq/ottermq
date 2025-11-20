package management

import "github.com/andrelcunha/ottermq/internal/core/models"

func (s *Service) ListConsumers(vhost string) ([]models.ConsumerDTO, error) {
	panic("not implemented")
}

func (s *Service) ListQueueConsumers(vhost, queue string) ([]models.ConsumerDTO, error) {
	panic("not implemented")
}
