package management

import "github.com/andrelcunha/ottermq/internal/core/models"

func (s *Service) GetOverview() (*models.OverviewDTO, error) {
	return &models.OverviewDTO{
		BrokerDetails:   s.broker.GetBrokerOverviewDetails(),
		NodeDetails:     s.broker.GetOverviewNodeDetails(),
		ObjectTotals:    s.broker.GetObjectTotalsOverview(),
		MessageStats:    s.GetMessageStats(),
		ConnectionStats: s.broker.GetOverviewConnStats(),
		Configuration:   s.broker.GetBrokerOverviewConfig(),
	}, nil
}

func (s *Service) GetMessageStats() models.OverviewMessageStats {
	// acumulate message stats from all vhosts
	vhosts := s.broker.ListVHosts()
	var mStats models.OverviewMessageStats
	qStats := []models.QueueMessageBreakdown{}
	for _, vh := range vhosts {
		dtos := s.getQueueDTOs(vh)
		for _, dto := range dtos {
			mStats.MessagesReady += dto.MessagesReady
			mStats.MessagesUnacked += dto.MessagesUnacked
			mStats.MessagesTotal += dto.MessagesTotal
			qStats = append(qStats, models.QueueMessageBreakdown{
				VHost:           vh.Name,
				QueueName:       dto.Name,
				MessagesReady:   dto.MessagesReady,
				MessagesUnacked: dto.MessagesUnacked,
			})
		}
	}
	return mStats
}
