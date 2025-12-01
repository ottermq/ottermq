package management

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

// ListBindings lists all bindings across all vhosts.
func (s *Service) ListBindings() ([]models.BindingDTO, error) {
	bindingDtos := make([]models.BindingDTO, 0)
	for _, vh := range s.broker.ListVHosts() {
		bindingDtos = collectExchangeBindings(vh, bindingDtos)
	}
	return bindingDtos, nil
}

// ListVhostBindings lists all bindings in the specified vhost.
func (s *Service) ListVhostBindings(vhostName string) ([]models.BindingDTO, error) {
	vh := s.broker.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhostName)
	}

	bindingDtos := collectExchangeBindings(vh, make([]models.BindingDTO, 0))

	return bindingDtos, nil
}

// collectExchangeBindings collects all bindings from all exchanges in the given vhost.
func collectExchangeBindings(vh *vhost.VHost, bindingDtos []models.BindingDTO) []models.BindingDTO {
	exchanges := vh.GetAllExchanges()
	for _, exchange := range exchanges {
		bindingDtos = listExchangeBindings(exchange, vh.Name, bindingDtos)
	}
	return bindingDtos
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

func (s *Service) ListQueueBindings(vhost, queueName string) ([]models.BindingDTO, error) {
	vh := s.broker.GetVHost(vhost)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", vhost)
	}

	queue := vh.GetQueue(queueName)
	if queue == nil {
		return nil, fmt.Errorf("queue '%s' not found in vhost '%s'", queueName, vhost)
	}

	var bindingsList []models.BindingDTO

	exchanges := vh.GetAllExchanges()
	for _, exchange := range exchanges {
		exchangeBindings := exchange.GetBindings()
		for _, bindings := range exchangeBindings {
			for _, binding := range bindings {
				if binding.Queue.Name == queue.Name {
					dto := models.BindingDTO{
						VHost:       vhost,
						Source:      exchange.NameOrAlias(),
						Destination: binding.Queue.Name,
						RoutingKey:  binding.RoutingKey,
						Arguments:   binding.Args,
					}
					bindingsList = append(bindingsList, dto)
				}
			}
		}
	}

	return bindingsList, nil
}

func (s *Service) CreateBinding(req models.CreateBindingRequest) (*models.BindingDTO, error) {
	vh := s.broker.GetVHost(req.VHost)
	if vh == nil {
		return nil, fmt.Errorf("vhost '%s' not found", req.VHost)
	}

	exchangeName := req.Source
	queue := req.Destination

	err := vh.BindQueue(exchangeName, queue, req.RoutingKey, req.Arguments, vhost.MANAGEMENT_CONNECTION_ID)
	if err != nil {
		return nil, err
	}
	exchange := vh.GetExchange(exchangeName)
	if exchange == nil {
		return nil, fmt.Errorf("exchange '%s' not found in vhost '%s'", exchangeName, req.VHost)
	}
	dto := &models.BindingDTO{
		VHost:       req.VHost,
		Source:      exchange.NameOrAlias(),
		Destination: queue,
		RoutingKey:  req.RoutingKey,
		Arguments:   req.Arguments,
	}
	return dto, nil
}

func (s *Service) DeleteBinding(req models.DeleteBindingRequest) error {
	vh := s.broker.GetVHost(req.VHost)
	if vh == nil {
		return fmt.Errorf("vhost '%s' not found", req.VHost)
	}

	exchange := req.Source
	queue := req.Destination

	return vh.UnbindQueue(exchange, queue, req.RoutingKey, req.Arguments, vhost.MANAGEMENT_CONNECTION_ID)
}
