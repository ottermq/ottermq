package broker

import (
	"fmt"
	"net"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/rs/zerolog/log"
)

const MaxTransactionBufferSize = 10000

// txHandler handles transaction-related commands
func (b *Broker) txHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	switch request.MethodID {
	case uint16(amqp.TX_SELECT):
		return b.txSelectHandler(request, vh, conn)

	case uint16(amqp.TX_COMMIT):
		return b.txCommitHandler(request, vh, conn)

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
	defer txState.Unlock()

	if txState.InTransaction {
		log.Debug().Uint16("channel", channel).Msg("Channel already in transaction mode")
		return nil, b.createTxSelectOkResponse(channel, conn)
	}

	// First time entering transaction mode
	txState.InTransaction = true
	txState.BufferedPublishes = []vhost.BufferedPublish{}
	txState.BufferedAcks = []vhost.BufferedAck{}

	log.Debug().Uint16("channel", channel).Msg("Channel entered transaction mode")
	return nil, b.createTxSelectOkResponse(channel, conn)
}

func (b *Broker) txCommitHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	channel := request.Channel

	txState := vh.GetOrCreateTransactionState(channel, conn)
	txState.Lock()
	defer txState.Unlock()

	if err := b.ensureInTransaction(txState); err != nil {
		return nil, err
	}

	// Dry run to check for errors before committing
	publishErrors := []error{}
	for _, pub := range txState.BufferedPublishes {
		exchange := pub.ExchangeName
		routingKey := pub.RoutingKey
		mandatory := pub.Mandatory
		// msg := &pub.Message
		hasRouting, err := vh.HasRoutingForMessage(exchange, routingKey)
		if err != nil {
			publishErrors = append(publishErrors, err)
			continue
		}

		if !hasRouting {
			if mandatory {
				log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("No route for message, would be returned to publisher")
				// publishErrors = append(publishErrors, fmt.Errorf("no route for message on exchange %s with routing key %s", exchange, routingKey))
				continue
			}
			// No routing and not mandatory - silently drop the message
			// log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("No route for message, would be silently dropped (not mandatory)")
			continue
		}
	}
	if len(publishErrors) > 0 {
		b.clearBufferedTransactionDataUnlocked(txState)
		log.Error().Msgf("Transaction commit dry run encountered errors: %v", publishErrors)
		return nil, fmt.Errorf("transaction commit failed")
	}

	// Execute all buffered publishes atomically
	for _, pub := range txState.BufferedPublishes {
		exchange := pub.ExchangeName
		routingKey := pub.RoutingKey
		mandatory := pub.Mandatory
		amqpMsg := &pub.Message
		hasRouting, err := vh.HasRoutingForMessage(exchange, routingKey)

		if !hasRouting {
			if mandatory {
				log.Debug().Str("exchange", exchange).Str("routing_key", routingKey).Msg("No route for message, returned to publisher")
				_, err = b.BasicReturn(conn, channel, exchange, routingKey, amqpMsg)
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
		msgID := vhost.GenerateMessageId()
		msg := vhost.NewMessage(*amqpMsg, msgID)
		_, err = vh.Publish(pub.ExchangeName, pub.RoutingKey, &msg)
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
		errMsg := fmt.Sprintf("Transaction commit failed: %d publish errors, %d ack errors",
			len(publishErrors), len(ackErrors))
		log.Error().
			Errs("publish_errors", publishErrors).
			Errs("ack_errors", ackErrors).
			Msg(errMsg)

		exception := errors.NewChannelError(
			errMsg,
			uint16(amqp.INTERNAL_ERROR),
			uint16(amqp.TX),
			uint16(amqp.TX_COMMIT),
		)
		return sendChannelErrorResponse(exception, b, conn, request)
	}

	// Clear buffered operations
	b.clearBufferedTransactionDataUnlocked(txState)
	return nil, b.createTxCommitOkResponse(channel, conn)
}

func (*Broker) clearBufferedTransactionDataUnlocked(txState *vhost.ChannelTransactionState) {
	txState.BufferedPublishes = []vhost.BufferedPublish{}
	txState.BufferedAcks = []vhost.BufferedAck{}
}

func (b *Broker) txRollbackHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	channel := request.Channel
	txState := vh.GetOrCreateTransactionState(channel, conn)
	txState.Lock()
	defer txState.Unlock()

	if err := b.ensureInTransaction(txState); err != nil {
		return nil, err
	}

	txState.BufferedPublishes = []vhost.BufferedPublish{}
	txState.BufferedAcks = []vhost.BufferedAck{}

	log.Debug().Uint16("channel", channel).Msg("Channel transaction rolled back")
	return nil, b.createTxRollbackOkResponse(channel, conn)
}

func (*Broker) ensureInTransaction(txState *vhost.ChannelTransactionState) error {
	if txState == nil || !txState.InTransaction {
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
