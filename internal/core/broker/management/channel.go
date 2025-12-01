package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// ListChannels returns all channels across all connections.
func (s *Service) ListChannels(vhost string) ([]models.ChannelDetailDTO, error) {
	if vhost != "" {
		vh := s.broker.GetVHost(vhost)
		if vh == nil {
			return nil, fmt.Errorf("vhost '%s' not found", vhost)
		}
	}
	channelsInfo, err := s.broker.ListChannels(vhost)
	if err != nil {
		return nil, err
	}

	// Map to DTOs
	channels := make([]models.ChannelDetailDTO, 0, len(channelsInfo))
	for _, chInfo := range channelsInfo {

		chDTO := mapChannelInfoToDTO(chInfo)
		channels = append(channels, chDTO)
	}

	return channels, nil
}

func (s *Service) ListConnectionChannels(name string) ([]models.ChannelDetailDTO, error) {

	channelsInfo, err := s.broker.ListConnectionChannels(name)
	if err != nil {
		return nil, err
	}

	// Map to DTOs
	channels := make([]models.ChannelDetailDTO, 0, len(channelsInfo))
	for _, chInfo := range channelsInfo {

		chDTO := mapChannelInfoToDTO(chInfo)
		channels = append(channels, chDTO)
	}

	return channels, nil
}

func mapChannelInfoToDTO(chInfo models.ChannelInfo) models.ChannelDetailDTO {
	chDTO := models.ChannelDetailDTO{
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

func (s *Service) GetChannel(connectionName string, channelNumber uint16) (*models.ChannelDetailDTO, error) {
	vh := s.broker.GetVHost("/")
	if vh == nil {
		return nil, nil
	}
	connInfo := vhost.ConnectionID(connectionName)
	chInfo, err := s.broker.CreateChannelInfo(connInfo, channelNumber, vh)
	if err != nil {
		return nil, err
	}

	chDTO := mapChannelInfoToDTO(chInfo)
	return &chDTO, nil
}
