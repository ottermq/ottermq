package broker

import (
	"fmt"
	"net"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/rs/zerolog/log"
)

// txHandler handles transaction-related commands
func (b *Broker) txHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	switch request.MethodID {
	case uint16(amqp.TX_SELECT):
		return b.txSelectHandler(request, vh, conn)
	case uint16(amqp.TX_COMMIT):
		// Handle commit asynchronously to avoid blocking the main loop
		// the acknowledgment shall be sent immediately
		if err := b.createTxCommitOkResponse(request.Channel, conn); err != nil {
			return nil, err
		}

		go func() {
			b.txCommitHandler(request, vh, conn)
		}()
		return nil, nil

	case uint16(amqp.TX_ROLLBACK):
		return b.txRollbackHandler(request, vh, conn)
	default:
		return nil, fmt.Errorf("unsupported command")
	}
}

func (b *Broker) txSelectHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	channel := request.Channel

	txState := vh.GetOrCreateTransactionState(channel, conn)
	txState.Lock()
	if txState.InTransaction {
		txState.Unlock()
		return nil, b.createTxSelectOkResponse(channel, conn)
	}

	txState.InTransaction = true
	txState.BufferedPublishes = []vhost.BufferedPublish{}
	txState.BufferedAcks = []vhost.BufferedAck{}
	txState.Unlock()

	log.Debug().Uint16("channel", channel).Msg("Channel entered transaction mode")
	return nil, b.createTxSelectOkResponse(channel, conn)
}

func (b *Broker) txCommitHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	channel := request.Channel

	txState := vh.GetOrCreateTransactionState(channel, conn)
	txState.Lock()

	if err := b.ensureInTransaction(txState); err != nil {
		return nil, err
	}
	// Execute all buffered publishes atomically
	publishErrors := []error{}
	for _, pub := range txState.BufferedPublishes {
		exchange := pub.ExchangeName
		routingKey := pub.RoutingKey
		mandatory := pub.Mandatory
		msg := &pub.Message
		hasRouting, err := vh.HasRoutingForMessage(exchange, routingKey)

		if !hasRouting {
			if mandatory {
				log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("No route for message, returned to publisher")
				_, err = b.BasicReturn(conn, channel, exchange, routingKey, msg)
				if err != nil {
					publishErrors = append(publishErrors, err)
				} else {
					publishErrors = append(publishErrors, fmt.Errorf("no route for message on exchange %s with routing key %s", exchange, routingKey))
				}
				continue
			}
			// No routing and not mandatory - silently drop the message
			log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("No route for message, silently dropped (not mandatory)")
			b.Connections[conn].Channels[channel] = &amqp.ChannelState{}
			continue
		}

		// TODO: consider a dry-run publish to validate before actual publish
		_, err = vh.Publish(pub.ExchangeName, pub.RoutingKey, msg)
		if err != nil {
			publishErrors = append(publishErrors, err)
		}
	}

	// Execute all buffered acks atomically
	ackErrors := []error{}
	for _, ack := range txState.BufferedAcks {
		var err error
		switch ack.Operation {
		case vhost.AckOperationAck:
			err = vh.HandleBasicAck(conn, channel, ack.DeliveryTag, ack.Multiple)
		case vhost.AckOperationNack:
			err = vh.HandleBasicNack(conn, channel, ack.DeliveryTag, ack.Multiple, ack.Requeue)
		case vhost.AckOperationReject:
			err = vh.HandleBasicReject(conn, channel, ack.DeliveryTag, ack.Requeue)
		}
		if err != nil {
			ackErrors = append(ackErrors, err)
		}
	}

	// If there were any errors, we need to rollback the transaction
	if len(publishErrors) > 0 || len(ackErrors) > 0 {
		// Rollback: This is tricky - already published messages can't be unpublished
		// For MVP: Log error and treat as partial commit (matches RabbitMQ behavior for some errors)
		// For production: Use 2-phase commit or WAL

		txState.Unlock()
		log.Error().Msgf("Transaction commit encountered errors: %v %v", publishErrors, ackErrors)
		return nil, fmt.Errorf("transaction commit failed")
	}

	// Clear buffered operations
	txState.BufferedPublishes = []vhost.BufferedPublish{}
	txState.BufferedAcks = []vhost.BufferedAck{}
	txState.InTransaction = false
	txState.Unlock()
	return nil, nil
}

func (b *Broker) txRollbackHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	channel := request.Channel
	txState := vh.GetOrCreateTransactionState(channel, conn)
	txState.Lock()

	if err := b.ensureInTransaction(txState); err != nil {
		return nil, err
	}

	txState.BufferedPublishes = []vhost.BufferedPublish{}
	txState.BufferedAcks = []vhost.BufferedAck{}
	txState.InTransaction = false
	txState.Unlock()

	log.Debug().Uint16("channel", channel).Msg("Channel transaction rolled back")
	return nil, b.createTxRollbackOkResponse(channel, conn)
}

func (*Broker) ensureInTransaction(txState *vhost.ChannelTransactionState) error {
	if txState == nil || !txState.InTransaction {
		txState.Unlock()
		// raise channel exception
		return errors.NewChannelError(
			"not in transaction mode",
			uint16(amqp.PRECONDITION_FAILED),
			uint16(amqp.TX),
			uint16(amqp.TX_ROLLBACK),
		)
	}
	return nil
}

func (b *Broker) createTxSelectOkResponse(channel uint16, conn net.Conn) error {
	frame := b.framer.CreateTxSelectOkFrame(channel)
	return b.framer.SendFrame(conn, frame)
}

func (b *Broker) createTxCommitOkResponse(channel uint16, conn net.Conn) error {
	frame := b.framer.CreateTxCommitOkFrame(channel)
	return b.framer.SendFrame(conn, frame)
}

func (b *Broker) createTxRollbackOkResponse(channel uint16, conn net.Conn) error {
	frame := b.framer.CreateTxRollbackOkFrame(channel)
	return b.framer.SendFrame(conn, frame)
}
