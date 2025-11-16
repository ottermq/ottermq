package vhost

import (
	"github.com/andrelcunha/ottermq/pkg/persistence"
)

func (vh *VHost) RecoverExchange(name, typ string, props persistence.ExchangeProperties, bindings []persistence.BindingData) {
	ex := &Exchange{
		Name:     name,
		Typ:      ExchangeType(typ),
		Bindings: make(map[string][]*Binding),
		Props: &ExchangeProperties{
			Durable:    props.Durable,
			AutoDelete: props.AutoDelete,
			Internal:   props.Internal,
			Arguments:  props.Arguments,
		},
	}

	for _, binding := range bindings {
		queue, ok := vh.Queues[binding.QueueName]
		if !ok {
			continue
		}
		ex.Bindings[binding.RoutingKey] = append(ex.Bindings[binding.RoutingKey], &Binding{
			Queue:      queue,
			RoutingKey: binding.RoutingKey,
			Args:       binding.Arguments,
		})
	}

	vh.Exchanges[ex.Name] = ex
}

// RecoverQueue recreates a queue from its persisted state
func (vh *VHost) RecoverQueue(name string, props *persistence.QueueProperties) error {
	q := &Queue{
		Name: name,
		Props: &QueueProperties{
			Durable:    props.Durable,
			Exclusive:  props.Exclusive,
			AutoDelete: props.AutoDelete,
			Arguments:  props.Arguments,
		},
	}
	vh.Queues[q.Name] = q
	// Recreate messages
	msgs, err := vh.persist.LoadMessages(vh.Name, name)
	if err != nil {
		// log.Error().Err(err).Str("queue", name).Msg("Failed to load messages")
		return err
	}

	for _, msgData := range msgs {
		msg := Message{
			ID:   msgData.ID,
			Body: msgData.Body,
			// TODO: map properties
		}
		select {
		case q.messages <- msg:
			q.count++
		default:
			// Optionally handle the case where the channel is full
		}
	}
	return nil
}
