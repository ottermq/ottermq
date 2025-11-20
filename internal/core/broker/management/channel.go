package management

import "github.com/andrelcunha/ottermq/internal/core/models"

func (s *Service) ListChannels() ([]models.ChannelDTO, error) {
	panic("not implemented")
}

func (s *Service) ListConnectionChannels(name string) ([]models.ChannelDTO, error) {
	panic("not implemented")
}

func (s *Service) GetChannel(connectionName string, channelNumber uint16) (*models.ChannelDTO, error) {
	panic("not implemented")
}
