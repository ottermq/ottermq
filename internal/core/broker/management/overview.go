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
	snapshot := *collector.GetBrokerSnapshot()
	duration := time.Duration(60 * time.Second) // last 60 seconds

	// Convert to time-series DTOs with rates
	return &models.OverviewChartsDTO{
		MessageStats: models.MessageStatsTimeSeriesDTO{
			Ready:   samplesToTimeSeries(snapshot.TotalReadyDepth.GetSamples()),
			Unacked: samplesToTimeSeries(snapshot.TotalUnackedDepth.GetSamples()),
			Total:   samplesToTimeSeries(snapshot.TotalDepth.GetSamples()),
		},
		MessageRates: models.MessageRatesTimeSeriesDTO{
			Publish: samplesToRates(collector.GetPublishRateTimeSeries(duration)),
			Deliver: samplesToRates(collector.GetDeliveryRateTimeSeries(duration)),
			Ack:     samplesToRates(collector.GetAckRateTimeSeries(duration)),
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
func samplesToRates(samples []metrics.Sample) []models.TimeSeriesDTO {
	if len(samples) < 2 {
		return []models.TimeSeriesDTO{}
	}

	result := make([]models.TimeSeriesDTO, 0, len(samples)-1)

	for i := 1; i < len(samples); i++ {
		prev := samples[i-1]
		curr := samples[i]

		elapsed := curr.Timestamp.Sub(prev.Timestamp).Seconds()
		log.Debug().Str("current", curr.Timestamp.String()).Str("previous", prev.Timestamp.String()).Msg("Getting timestamps")
		log.Debug().Float64("elapsed", elapsed).Msg("Calculating rate")
		if elapsed > 0 {
			log.Debug().Int64("prevCount", prev.Count).Int64("currCount", curr.Count).Msg("Calculating rate counts")
			rate := float64(curr.Count-prev.Count) / elapsed
			result = append(result, models.TimeSeriesDTO{
				Timestamp: curr.Timestamp,
				Value:     rate,
			})
		}
	}

	return result
}
