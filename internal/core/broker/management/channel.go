package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/andrelcunha/ottermq/pkg/metrics"
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
	collector := s.broker.GetCollector()
	// Map to DTOs
	channels := make([]models.ChannelDetailDTO, 0, len(channelsInfo))
	for _, chInfo := range channelsInfo {
		chanSnapshot := collector.GetChannelSnapshot(chInfo.ConnectionName, vhost, chInfo.Number)
		chDTO := mapChannelSnapshotToDTO(chanSnapshot)
		channels = append(channels, chDTO)
	}

	return channels, nil
}

func (s *Service) ListConnectionChannels(name string) ([]models.ChannelDetailDTO, error) {

	channelsInfo, err := s.broker.ListConnectionChannels(name)
	if err != nil {
		return nil, err
	}
	collector := s.broker.GetCollector()

	// Map to DTOs
	channels := make([]models.ChannelDetailDTO, 0, len(channelsInfo))
	for _, chInfo := range channelsInfo {

		chanSnapshot := collector.GetChannelSnapshot(chInfo.ConnectionName, chInfo.VHost, chInfo.Number)
		chDTO := mapChannelSnapshotToDTO(chanSnapshot)
		channels = append(channels, chDTO)
	}

	return channels, nil
}

// func mapChannelInfoToDTO(chInfo models.ChannelInfo) models.ChannelDetailDTO {
// 	chDTO := models.ChannelDetailDTO{
// 		Number:           chInfo.Number,
// 		ConnectionName:   chInfo.ConnectionName,
// 		VHost:            chInfo.VHost,
// 		State:            chInfo.State,
// 		UnconfirmedCount: chInfo.UnconfirmedCount,
// 		PrefetchCount:    chInfo.PrefetchCount,
// 		UnackedCount:     chInfo.UnackedCount,
// 		PublishRate:      0, // TODO: implement publish rate calculation
// 		DeliverRate:      0, // TODO: implement deliver rate calculation
// 		UnroutableRate:   0, // TODO: implement unroutable rate calculation
// 		AckRate:          0,
// 	}
// 	return chDTO
// }

func mapChannelSnapshotToDTO(chSnapshot *metrics.ChannelSnapshot) models.ChannelDetailDTO {
	chDTO := models.ChannelDetailDTO{
		Number:           chSnapshot.ChannelNumber,
		ConnectionName:   chSnapshot.ConnectionName,
		VHost:            chSnapshot.VHostName,
		State:            chSnapshot.State, // TODO: determine actual state
		UnconfirmedCount: 0,
		PrefetchCount:    uint16(chSnapshot.PrefetchCount),
		UnackedCount:     int(chSnapshot.UnackedCount),
		PublishRate:      chSnapshot.PublishRate,
		DeliverRate:      chSnapshot.DeliverRate,
		UnroutableRate:   chSnapshot.UnroutableRate,
		AckRate:          chSnapshot.AckRate,
	}
	return chDTO
}

func (s *Service) GetChannel(connectionName string, channelNumber uint16) (*models.ChannelDetailDTO, error) {
	vh := s.broker.GetVHost("/")
	if vh == nil {
		return nil, nil
	}
	// chInfo, err := s.broker.CreateChannelInfo(connInfo, channelNumber, vh)
	// if err != nil {
	// 	return nil, err
	// }
	collector := s.broker.GetCollector()
	chanSnapshot := collector.GetChannelSnapshot(connectionName, vh.Name, channelNumber)
	chDTO := mapChannelSnapshotToDTO(chanSnapshot)
	return &chDTO, nil
}
