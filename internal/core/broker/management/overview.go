package management

import (
	"time"

	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/andrelcunha/ottermq/pkg/metrics"
	"github.com/rs/zerolog/log"
)

func (s *Service) GetOverview() (*models.OverviewDTO, error) {
	collector := s.broker.GetCollector()
	snapshot := *collector.GetBrokerSnapshot()

	// Compress the Prometheus metrics using Snappy
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

// GetOverviewCharts returns time-series data for overview page charts
func (s *Service) GetOverviewCharts() (*models.OverviewChartsDTO, error) {
	collector := s.broker.GetCollector()
	bm := collector.GetBrokerMetrics()
	duration := time.Duration(60 * time.Second) // last 60 seconds

	// Convert to time-series DTOs with rates
	return &models.OverviewChartsDTO{
		MessageStats: models.MessageStatsTimeSeriesDTO{
			Ready:   samplesToTimeSeries(bm.TotalReadyDepth().GetSamplesForDuration(duration)),
			Unacked: samplesToTimeSeries(bm.TotalUnackedDepth().GetSamplesForDuration(duration)),
			Total:   samplesToTimeSeries(bm.TotalDepth().GetSamplesForDuration(duration)),
		},
		MessageRates: models.MessageRatesTimeSeriesDTO{
			// using 0 duration to get all samples and filter in samplesToRates
			// to avoid gaps in the rate calculations
			Publish:          samplesToRates(collector.GetPublishRateTimeSeries(0)),
			DeliverAutoAck:   samplesToRates(collector.GetDeliveryAutoAckRateTimeSeries(0)),
			DeliverManualAck: samplesToRates(collector.GetDeliveryManualAckRateTimeSeries(0)),
			Ack:              samplesToRates(collector.GetAckRateTimeSeries(0)),
		},
	}, nil
}

// samplesToTimeSeries converts raw samples to TimeSeriesDTO
func samplesToTimeSeries(samples []metrics.Sample) []models.TimeSeriesDTO {
	result := make([]models.TimeSeriesDTO, len(samples))
	for i, s := range samples {
		result[i] = models.TimeSeriesDTO{
			Timestamp: s.Timestamp,
			Value:     float64(s.Count),
		}
	}
	return result
}

// samplesToRates converts cumulative count samples to rate-per-second values
// FIXED: Now filters AFTER calculating rates to avoid gaps
func samplesToRates(samples []metrics.Sample) []models.TimeSeriesDTO {
	if len(samples) < 2 {
		return []models.TimeSeriesDTO{}
	}

	result := make([]models.TimeSeriesDTO, 0, len(samples)-1)
	cutoff := time.Now().Add(-60 * time.Second) // Filter to last 60 seconds

	for i := 1; i < len(samples); i++ {
		prev := samples[i-1]
		curr := samples[i]

		elapsed := curr.Timestamp.Sub(prev.Timestamp).Seconds()

		// Only include if current timestamp is within our window
		if curr.Timestamp.After(cutoff) && elapsed > 0 {
			rate := float64(curr.Count-prev.Count) / elapsed

			// Clamp negative rates to 0 (shouldn't happen with consecutive samples)
			if rate < 0 {
				log.Warn().
					Int64("prevCount", prev.Count).
					Int64("currCount", curr.Count).
					Time("prevTime", prev.Timestamp).
					Time("currTime", curr.Timestamp).
					Msg("Negative rate detected - counter decreased")
				rate = 0
			}

			result = append(result, models.TimeSeriesDTO{
				Timestamp: curr.Timestamp,
				Value:     rate,
			})
		}
	}

	return result
}
