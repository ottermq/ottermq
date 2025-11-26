package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// ListExchanges lists all exchanges in the specified vhost.
func (s *Service) ListExchanges() ([]models.ExchangeDTO, error) {
	dtos := make([]models.ExchangeDTO, 0)
	for _, vh := range s.broker.ListVHosts() {
		for _, exchange := range vh.GetAllExchanges() {
			dto := models.ExchangeDTO{
				VHost: vh.Name,
				Name:  exchange.NameOrAlias(),
				Type:  string(exchange.Typ),
			}
			dtos = append(dtos, dto)
		}
	}
	return dtos, nil
}

// GetExchange retrieves details of a specific exchange in the specified vhost.
func (s *Service) GetExchange(vhostName, exchangeName string) (*models.ExchangeDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	exchange := vh.GetExchange(exchangeName)
	if exchange == nil {
		return nil, fmt.Errorf("exchange '%s' not found in vhost '%s'", exchangeName, vhostName)
	}

	dto := s.exchangeToDTO(vh, exchange)

	return dto, nil
}

// CreateExchange creates a new exchange in the specified vhost.
func (s *Service) CreateExchange(vhostName, exchangeName string, req models.CreateExchangeRequest) (*models.ExchangeDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}
	exchangeType := vhost.ExchangeType(req.ExchangeType)
	props := vhost.ExchangeProperties{
		Passive:    req.Passive,
		Durable:    req.Durable,
		AutoDelete: req.AutoDelete,
		Internal:   req.Internal,
		Arguments:  req.Arguments,
	}
	err := vh.CreateExchange(exchangeName, exchangeType, &props)
	if err != nil {
		return nil, err
	}

	exchange := vh.GetExchange(exchangeName)
	dto := s.exchangeToDTO(vh, exchange)
	return dto, nil
}

// DeleteExchange deletes an exchange from the specified vhost.
func (s *Service) DeleteExchange(vhostName, exchangeName string, ifUnused bool) error {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return fmt.Errorf("vhost '%s' not found", vhostName)
	}
	exchange := vh.GetExchange(exchangeName)
	if exchange == nil {
		return nil // Idempotent delete
	}
	if ifUnused {
		if exchange.BindingCount() > 0 {
			return fmt.Errorf("exchange '%s' has active bindings", exchangeName)
		}
	}

	return vh.DeleteExchange(exchangeName)
}

func (*Service) exchangeToDTO(vh *vhost.VHost, exchange *vhost.Exchange) *models.ExchangeDTO {

	dto := &models.ExchangeDTO{
		VHost:           vh.Name,
		Name:            exchange.NameOrAlias(),
		Type:            string(exchange.Typ),
		Durable:         exchange.Props.Durable,
		AutoDelete:      exchange.Props.AutoDelete,
		Internal:        exchange.Props.Internal,
		Arguments:       exchange.Props.Arguments,
		MessageStatsIn:  &models.MessageStats{},
		MessageStatsOut: &models.MessageStats{},
	}
	// TODO: add message stats

	return dto
}
