package broker

import (
	"fmt"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
)

type Alias map[string](string)

const DEFAULT_EXCHANGE_ALIAS = "(AMQP default)"

var defaultAlias Alias = Alias{
	vhost.DEFAULT_EXCHANGE: DEFAULT_EXCHANGE_ALIAS,
	vhost.EMPTY_EXCHANGE:   DEFAULT_EXCHANGE_ALIAS,
	DEFAULT_EXCHANGE_ALIAS: vhost.DEFAULT_EXCHANGE,
}

type ManagerApi interface {
	ListExchanges() []models.ExchangeDTO
	CreateExchange(dto models.ExchangeDTO) error
	GetExchange(vhostName, exchangeName string) (*vhost.Exchange, error)
	GetExchangeUniqueNames() map[string]bool
	ListQueues() ([]models.QueueDTO, error)
	DeleteQueue(vhostName, queueName string) error
	GetTotalQueues() int
	ListConnections() []models.ConnectionInfoDTO
	ListBindings(vhostName, exchangeName string) map[string][]string
}

func NewDefaultManagerApi(broker *Broker) *DefaultManagerApi {
	return &DefaultManagerApi{broker: broker}
}

type DefaultManagerApi struct {
	broker *Broker
}

func (a DefaultManagerApi) ListExchanges() []models.ExchangeDTO {
	b := a.broker
	names := a.GetExchangeUniqueNames()
	exchanges := make([]models.ExchangeDTO, 0, len(names))
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, vh := range b.VHosts {
		for xn := range names {
			if exchange, ok := vh.Exchanges[xn]; ok {
				exchanges = append(exchanges, models.ExchangeDTO{
					VHostName: vh.Name,
					Name: func(xname string) string {
						if alias, ok := defaultAlias[xname]; ok {
							return alias
						}
						return xname
					}(xn),
					Type: string(exchange.Typ),
				})
			}
		}
	}
	return exchanges
}

func (a DefaultManagerApi) GetExchange(vhostName, exchangeName string) (*vhost.Exchange, error) {
	b := a.broker
	vh := b.GetVHost(vhostName)
	if vh == nil {
		return nil, fmt.Errorf("vhost %s not found", vhostName)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		return nil, fmt.Errorf("exchange %s not found in vhost %s", exchangeName, vhostName)
	}
	return exchange, nil
}

func (a DefaultManagerApi) CreateExchange(dto models.ExchangeDTO) error {
	b := a.broker
	vh := b.GetVHost(dto.VHostName)
	if vh == nil {
		return fmt.Errorf("vhost %s not found", dto.VHostName)
	}
	typ, err := vhost.ParseExchangeType(dto.Type)
	if err != nil {
		return err
	}
	// prevent creating exchanges with the name of default aliases
	if _, ok := defaultAlias[dto.Name]; ok {
		return fmt.Errorf("cannot create exchange with reserved alias name '%s'", dto.Name)
	}
	// For simplicity, we are not allowing to set properties via the API for now
	// They will be set to default values
	return vh.CreateExchange(dto.Name, typ, nil)
}

func (a DefaultManagerApi) GetExchangeUniqueNames() map[string]bool {
	b := a.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	exchangeNames := make(map[string]bool)
	for _, vh := range b.VHosts {
		for _, exchange := range vh.Exchanges {
			if exchange.Name != vhost.EMPTY_EXCHANGE {
				exchangeNames[exchange.Name] = true
			}
		}
	}
	return exchangeNames
}

func (a DefaultManagerApi) ListQueues() ([]models.QueueDTO, error) {
	b := a.broker
	queues := make([]models.QueueDTO, 0, a.GetTotalQueues())
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, vh := range b.VHosts {
		unacked := vh.GetUnackedMessageCountsAllQueues()
		for _, queue := range vh.Queues {
			queues = append(queues, models.QueueDTO{
				VHostName: vh.Name,
				VHostId:   vh.Id,
				Name:      queue.Name,
				Messages:  queue.Len(),
				Unacked:   unacked[queue.Name],
			})
		}
	}
	return queues, nil
}

func (a DefaultManagerApi) DeleteQueue(vhostName, queueName string) error {
	b := a.broker
	vh := b.GetVHost(vhostName)
	if vh == nil {
		return fmt.Errorf("vhost %s not found", vhostName)
	}
	return vh.DeleteQueue(queueName)
}

func (a DefaultManagerApi) GetTotalQueues() int {
	b := a.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	total := 0
	for _, vh := range b.VHosts {
		for _, queue := range vh.Queues {
			if queue.Name != "" {
				total++
			}
		}
	}
	return total
}

func (a DefaultManagerApi) ListConnections() []models.ConnectionInfoDTO {
	b := a.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	connections := make([]amqp.ConnectionInfo, 0, len(b.Connections))
	for _, c := range b.Connections {
		connections = append(connections, *c)
	}
	connectionsDTO := models.MapListConnectionsDTO(connections)
	return connectionsDTO
}

func (a DefaultManagerApi) ListBindings(vhostName, exchangeName string) map[string][]string {
	b := a.broker
	vh := b.GetVHost(vhostName)
	b.mu.Lock()
	defer b.mu.Unlock()
	if vh == nil {
		return nil
	}
	// get the exchange from alias if it exists
	if realName, ok := defaultAlias[exchangeName]; ok {
		exchangeName = realName
	}
	fmt.Printf("exchangeName: %s, Vhost: %s", exchangeName, vh.Name)
	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		return nil
	}

	switch exchange.Typ {
	case vhost.DIRECT:
		bindings := make(map[string][]string)
		for routingKey, bs := range exchange.Bindings {
			var queuesStr []string
			for _, binding := range bs {
				queuesStr = append(queuesStr, binding.Queue.Name)
			}
			bindings[routingKey] = queuesStr
		}
		return bindings
	case vhost.FANOUT:
		bindings := make(map[string][]string)
		var queues []string
		for _, b := range exchange.Bindings[""] {
			queues = append(queues, b.Queue.Name)
		}
		bindings[""] = queues
		return bindings
	case vhost.TOPIC:
		// not implemented

	}
	return nil
}
