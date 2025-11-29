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
	if s.broker == nil {
		return nil, fmt.Errorf("broker not initialized")
	}
	amqpConn, err := s.broker.GetConnectionByName(name)
	if err != nil {
		return nil, err
	}
	if amqpConn == nil {
		return nil, fmt.Errorf("connection '%s' not found", name)
	}
	dto := models.MapConnectionInfoDTO(*amqpConn)
	return &dto, nil
}

func (s *Service) CloseConnection(name string, reason string) error {
	if s.broker == nil {
		return fmt.Errorf("broker not initialized")
	}
	return s.broker.CloseConnection(name, reason)
}
