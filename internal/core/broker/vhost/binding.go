package vhost

import (
	"fmt"
	"reflect"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/rs/zerolog/log"
)

type Binding struct {
	Queue      *Queue
	RoutingKey string
	Args       map[string]any
}

// bindToDefaultExchange binds a queue to the default exchange using the queue name as the routing key.
func (vh *VHost) BindToDefaultExchange(queueName string) error {
	return vh.BindQueue(DEFAULT_EXCHANGE, queueName, queueName, nil)
}

// BindQueue binds a queue to an exchange with a given routing key.
func (vh *VHost) BindQueue(exchangeName, queueName, routingKey string, args map[string]any) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	// Find the exchange
	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		return errors.NewChannelError(fmt.Sprintf("no exchange '%s' in vhost '%s'", exchangeName, vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.QUEUE), uint16(amqp.QUEUE_BIND))
	}
	queue, ok := vh.Queues[queueName]
	if !ok {
		return errors.NewChannelError(fmt.Sprintf("no queue '%s' in vhost '%s'", queueName, vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.QUEUE), uint16(amqp.QUEUE_BIND))
	}
	newBinding := &Binding{
		Queue:      queue,
		RoutingKey: routingKey,
		Args:       args,
	}
	switch exchange.Typ {
	case DIRECT:
		for _, b := range exchange.Bindings[routingKey] {
			// verify binding uniqueness
			if vh.bindingExist(b, queueName, routingKey, args, exchangeName) {
				return errors.NewChannelError(fmt.Sprintf("queue '%s' is already bound to exchange '%s' with routing key '%s'", queueName, exchangeName, routingKey), uint16(amqp.PRECONDITION_FAILED), uint16(amqp.QUEUE), uint16(amqp.QUEUE_BIND))
			}
		}
		exchange.Bindings[routingKey] = append(exchange.Bindings[routingKey], newBinding)

	case FANOUT:
		routingKey = "" // ignore routing key for fanout exchanges
		for _, b := range exchange.Bindings[routingKey] {
			// verify binding uniqueness
			if vh.bindingExist(b, queueName, routingKey, args, exchangeName) {
				return errors.NewChannelError(fmt.Sprintf("queue '%s' is already bound to exchange '%s'", queueName, exchangeName), uint16(amqp.PRECONDITION_FAILED), uint16(amqp.QUEUE), uint16(amqp.QUEUE_BIND))
			}
		}

		exchange.Bindings[routingKey] = append(exchange.Bindings[routingKey], newBinding)
	}

	if exchange.Props.Durable && queue.Props.Durable {
		err := vh.persist.SaveBindingState(vh.Name, exchangeName, queueName, routingKey, exchange.Props.Arguments)
		if err != nil {
			log.Printf("Failed to save binding state: %v", err)
		}
	}
	return nil
}

func (*VHost) bindingExist(b *Binding, queueName string, routingKey string, args map[string]interface{}, exchangeName string) bool {
	if b.Queue.Name == queueName &&
		b.RoutingKey == routingKey &&
		bindingArgumentsMatch(b.Args, args) {
		log.Debug().Str("queue", queueName).Str("exchange", exchangeName).Str("routing_key", routingKey).Msg("Queue already bound to exchange")
		return true
	}
	return false
}

func (vh *VHost) UnbindQueue(exchangeName, queueName, routingKey string, args map[string]interface{}) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	// Find the exchange
	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		return errors.NewChannelError(fmt.Sprintf("no exchange '%s' in vhost '%s'", exchangeName, vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.QUEUE), uint16(amqp.QUEUE_UNBIND))
	}
	queue, ok := vh.Queues[queueName]
	if !ok {
		return errors.NewChannelError(fmt.Sprintf("no queue '%s' in vhost '%s'", queueName, vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.QUEUE), uint16(amqp.QUEUE_UNBIND))
	}

	// TODO: use args to identify the binding uniquely
	// if they didn't match, raise 406 (PRECONDITION_FAILED)

	// TODO: deal with queue exclusivity and raise 403 (ACCESS_REFUSED) if needed

	switch exchange.Typ {
	case DIRECT:
		err := vh.DeleteBindingUnlocked(exchange, queueName, routingKey, args)
		if err != nil {
			return err
		}

	case FANOUT:
		routingKey = "" // ignore routing key for fanout exchanges
		err := vh.DeleteBindingUnlocked(exchange, queueName, routingKey, args)
		if err != nil {
			return err
		}
	}

	if exchange.Props.Durable && queue.Props.Durable {
		// it is ignoring the args. TODO: use args to identify the binding uniquely
		err := vh.persist.DeleteBindingState(vh.Name, exchangeName, queueName, routingKey, args)
		if err != nil {
			log.Printf("Failed to delete binding state: %v", err)
		}
	}
	log.Debug().Str("queue", queueName).Str("exchange", exchange.Name).Str("routing_key", routingKey).Msg("Queue unbound from exchange")
	return nil
}

func (vh *VHost) DeleteBindingUnlocked(exchange *Exchange, queueName, routingKey string, args map[string]interface{}) error {
	queues, ok := exchange.Bindings[routingKey]
	if !ok {
		log.Printf("No bindings found for routing key '%s' in exchange '%s'", routingKey, exchange.Name)
		return errors.NewChannelError(fmt.Sprintf("no binding in vhost '%s'", vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.QUEUE), uint16(amqp.QUEUE_UNBIND))
	}

	// Find queue in the bindings
	var index int
	found := false
	for i, b := range queues {
		if b.Queue.Name == queueName &&
			bindingArgumentsMatch(b.Args, args) {
			index = i
			found = true
			break
		}
	}

	// This should not happen as we checked before, but just in case
	if !found {
		return errors.NewChannelError(fmt.Sprintf("no queue '%s' in vhost '%s'", queueName, vh.Name), uint16(amqp.NOT_FOUND), uint16(amqp.QUEUE), uint16(amqp.QUEUE_UNBIND))
	}

	// Remove the queue from the bindings
	exchange.Bindings[routingKey] = append(queues[:index], queues[index+1:]...)

	if len(exchange.Bindings[routingKey]) == 0 {
		delete(exchange.Bindings, routingKey)
		// Check if the exchange can be auto-deleted
		if deleted, err := vh.checkAutoDeleteExchangeUnlocked(exchange.Name); err != nil {
			log.Printf("Failed to check auto-delete exchange: %v", err)
			return err
		} else if deleted {
			log.Printf("Exchange %s was auto-deleted", exchange.Name)
		}
	}
	return nil
}

func bindingArgumentsMatch(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !reflect.DeepEqual(v, bv) {
			return false
		}
	}
	return true
}

func (vh *VHost) listFanoutQueues(exchangeName string) []string {
	vh.mu.Lock()
	defer vh.mu.Unlock()

	exchange, ok := vh.Exchanges[exchangeName]
	if !ok {
		return nil
	}

	var queues []string
	for _, binding := range exchange.Bindings[""] {
		queues = append(queues, binding.Queue.Name)
	}
	return queues
}
