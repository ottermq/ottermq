package management

import "github.com/andrelcunha/ottermq/internal/core/models"

func (s *Service) ListConnections() ([]models.ConnectionInfoDTO, error) {
	panic("not implemented")
}

func (s *Service) GetConnection(name string) (*models.ConnectionInfoDTO, error) {
	panic("not implemented")
}

func (s *Service) CloseConnection(name string, reason string) error {
	panic("not implemented")
}
