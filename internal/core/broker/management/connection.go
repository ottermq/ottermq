package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/models"
)

func (s *Service) ListConnections() ([]models.ConnectionInfoDTO, error) {
	if s.broker == nil {
		return nil, fmt.Errorf("broker not initialized")
	}
	amqpConns := s.broker.ListConnections()
	dtos := models.MapListConnectionsDTO(amqpConns)
	return dtos, nil
}

func (s *Service) GetConnection(name string) (*models.ConnectionInfoDTO, error) {
	panic("not implemented")
}

func (s *Service) CloseConnection(name string, reason string) error {
	panic("not implemented")
}
