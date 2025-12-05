package management

import "github.com/andrelcunha/ottermq/internal/core/models"

func (s *Service) GetOverview() (*models.OverviewDTO, error) {
	snapshot := *s.broker.GetCollector().GetBrokerMetrics().Snapshot()
	return &models.OverviewDTO{
		BrokerDetails:   s.broker.GetBrokerOverviewDetails(),
		NodeDetails:     s.broker.GetOverviewNodeDetails(),
		ObjectTotals:    s.broker.GetObjectTotalsOverview(),
		MessageStats:    s.GetMessageStats(),
		ConnectionStats: s.broker.GetOverviewConnStats(),
		Configuration:   s.broker.GetBrokerOverviewConfig(),
		Metrics:         snapshot,
	}, nil
}

func (s *Service) GetMessageStats() models.OverviewMessageStats {
	// acumulate message stats from all vhosts
	var mStats models.OverviewMessageStats
	qStats := []models.QueueMessageBreakdown{}
	dtos := s.ListQueues()
	for _, dto := range dtos {
		mStats.MessagesReady += dto.MessagesReady
		mStats.MessagesUnacked += dto.MessagesUnacked
		mStats.MessagesTotal += dto.MessagesTotal
		qStats = append(qStats, models.QueueMessageBreakdown{
			VHost:           dto.VHost,
			QueueName:       dto.Name,
			MessagesReady:   dto.MessagesReady,
			MessagesUnacked: dto.MessagesUnacked,
		})
	}

	mStats.QueueStats = qStats
	return mStats
}

// BasicBrokerInfo returns basic information about the broker.
func (s *Service) GetBrokerInfo() models.OverviewBrokerDetails {
	return s.broker.GetBrokerOverviewDetails()
}
