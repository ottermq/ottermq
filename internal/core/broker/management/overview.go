package management

import (
	"time"

	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/andrelcunha/ottermq/pkg/metrics"
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

	// Get current message stats for calculating time-series
	messageStats := s.GetMessageStats()

	// Get rate time-series from collector (these are cumulative counts)
	publishSamples := collector.GetPublishRateTimeSeries(5 * time.Minute)
	deliverySamples := collector.GetDeliveryRateTimeSeries(5 * time.Minute)

	// Get ACK samples from broker metrics
	brokerMetrics := collector.GetBrokerMetrics()
	ackSamples := brokerMetrics.TotalAcks().GetSamples()

	// Convert to time-series DTOs with rates
	return &models.OverviewChartsDTO{
		MessageStats: models.MessageStatsTimeSeriesDTO{
			Ready:   convertToTimeSeries(publishSamples, float64(messageStats.MessagesReady)),
			Unacked: convertToTimeSeries(publishSamples, float64(messageStats.MessagesUnacked)),
			Total:   convertToTimeSeries(publishSamples, float64(messageStats.MessagesTotal)),
		},
		MessageRates: models.MessageRatesTimeSeriesDTO{
			Publish: convertSamplesToRates(publishSamples),
			Deliver: convertSamplesToRates(deliverySamples),
			Ack:     convertSamplesToRates(ackSamples),
		},
	}, nil
}

// convertToTimeSeries creates a simple time series using the latest value
// This is a simplified approach - for accurate historical queue depths,
// we'd need to track them over time in the metrics collector
func convertToTimeSeries(samples []metrics.Sample, currentValue float64) []models.TimeSeriesDTO {
	if len(samples) == 0 {
		return []models.TimeSeriesDTO{}
	}

	result := make([]models.TimeSeriesDTO, len(samples))
	for i, sample := range samples {
		// Use current value as approximation for now
		// In a future enhancement, we could track actual queue depths over time
		result[i] = models.TimeSeriesDTO{
			Timestamp: sample.Timestamp,
			Value:     currentValue,
		}
	}
	return result
}

// convertSamplesToRates converts cumulative count samples to rate-per-second values
func convertSamplesToRates(samples []metrics.Sample) []models.TimeSeriesDTO {
	if len(samples) < 2 {
		return []models.TimeSeriesDTO{}
	}

	result := make([]models.TimeSeriesDTO, 0, len(samples)-1)

	for i := 1; i < len(samples); i++ {
		prev := samples[i-1]
		curr := samples[i]

		elapsed := curr.Timestamp.Sub(prev.Timestamp).Seconds()
		if elapsed > 0 {
			rate := float64(curr.Count-prev.Count) / elapsed
			result = append(result, models.TimeSeriesDTO{
				Timestamp: curr.Timestamp,
				Value:     rate,
			})
		}
	}

	return result
}
