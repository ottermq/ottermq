package management

import (
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// ListChannels returns all channels across all connections.
func (s *Service) ListChannels() ([]models.ChannelDTO, error) {
	vh := s.broker.GetVHost("/")
	if vh == nil {
		return nil, nil
	}

	channelsInfo, err := s.broker.ListChannels()
	if err != nil {
		return nil, err
	}

	// Map to DTOs
	channels := make([]models.ChannelDTO, 0, len(channelsInfo))
	for _, chInfo := range channelsInfo {

		chDTO := mapChannelInfoToDTO(chInfo)
		channels = append(channels, chDTO)
	}

	return channels, nil
}

func (s *Service) ListConnectionChannels(name string) ([]models.ChannelDTO, error) {
	vh := s.broker.GetVHost("/")
	if vh == nil {
		return nil, nil
	}

	channelsInfo, err := s.broker.ListConnectionChannels(name)
	if err != nil {
		return nil, err
	}

	// Map to DTOs
	channels := make([]models.ChannelDTO, 0, len(channelsInfo))
	for _, chInfo := range channelsInfo {

		chDTO := mapChannelInfoToDTO(chInfo)
		channels = append(channels, chDTO)
	}

	return channels, nil
}

func mapChannelInfoToDTO(chInfo models.ChannelInfo) models.ChannelDTO {
	chDTO := models.ChannelDTO{
		Number:           chInfo.Number,
		ConnectionName:   chInfo.ConnectionName,
		VHost:            chInfo.VHost,
		State:            chInfo.State,
		UnconfirmedCount: chInfo.UnconfirmedCount,
		PrefetchCount:    chInfo.PrefetchCount,
		UnackedCount:     chInfo.UnackedCount,
		PublishRate:      0, // TODO: implement publish rate calculation
		DeliverRate:      0, // TODO: implement deliver rate calculation
	}
	return chDTO
}

func (s *Service) GetChannel(connectionName string, channelNumber uint16) (*models.ChannelDTO, error) {
	panic("not implemented")
}
