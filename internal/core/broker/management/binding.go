package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

func (s *Service) ListBindings(vhostName string) ([]models.BindingDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	exchanges := vh.GetAllExchanges()
	bindingDtos := make([]models.BindingDTO, 0)
	for _, exchange := range exchanges {
		bindingDtos = listExchangeBindings(exchange, vhostName, bindingDtos)
	}

	return bindingDtos, nil
}

func (s *Service) ListExchangeBindings(vhostName string, exchangeName string) ([]models.BindingDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	exchange := vh.GetExchange(exchangeName)
	if exchange == nil {
		return nil, fmt.Errorf("exchange '%s' not found in vhost '%s'", exchangeName, vhostName)
	}

	bindingDtos := make([]models.BindingDTO, 0)
	bindingDtos = listExchangeBindings(exchange, vhostName, bindingDtos)

	return bindingDtos, nil
}

func listExchangeBindings(exchange *vhost.Exchange, vhostName string, bindingDtos []models.BindingDTO) []models.BindingDTO {
	exchangeBindings := exchange.GetBindings()
	for _, bindings := range exchangeBindings {
		for _, binding := range bindings {
			dto := models.BindingDTO{
				VHost:       vhostName,
				Source:      exchange.NameOrAlias(),
				Destination: binding.Queue.Name,
				RoutingKey:  binding.RoutingKey,
				Arguments:   binding.Args,
			}
			bindingDtos = append(bindingDtos, dto)
		}

	}
	return bindingDtos
}
