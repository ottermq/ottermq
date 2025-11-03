package broker

import (
	"fmt"
	"net"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/rs/zerolog/log"
)

func (b *Broker) exchangeHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	channel := request.Channel
	switch request.MethodID {
	case uint16(amqp.EXCHANGE_DECLARE):
		log.Debug().Interface("request", request).Msg("Received exchange declare request")
		log.Debug().Uint16("channel", channel).Msg("Channel")
		content, ok := request.Content.(*amqp.ExchangeDeclareMessage)
		if !ok {
			log.Error().Msg("Invalid content type for ExchangeDeclareMessage")
			return nil, fmt.Errorf("invalid content type for ExchangeDeclareMessage")
		}
		log.Debug().Interface("content", content).Msg("Content")
		exchangeName := content.ExchangeName
		exchangeType := vhost.ExchangeType(content.ExchangeType)
		props := vhost.ExchangeProperties{
			Passive:    content.Passive,
			Durable:    content.Durable,
			AutoDelete: content.AutoDelete,
			Internal:   content.Internal,
			NoWait:     content.NoWait,
			Arguments:  content.Arguments,
		}
		err := vh.CreateExchange(exchangeName, exchangeType, &props)
		if err != nil {
			return nil, err
		}
		frame := b.framer.CreateExchangeDeclareFrame(request.Channel)
		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send exchange declare frame")
		}
		return true, nil

	case uint16(amqp.EXCHANGE_DELETE):
		log.Debug().Interface("request", request).Msg("Received exchange.delete request")
		log.Debug().Uint16("channel", channel).Msg("Channel")
		content, ok := request.Content.(*amqp.ExchangeDeleteMessage)
		if !ok {
			log.Error().Msg("Invalid content type for ExchangeDeleteMessage")
			return nil, fmt.Errorf("invalid content type for ExchangeDeleteMessage")
		}
		log.Debug().Interface("content", content).Msg("Content")
		exchangeName := content.ExchangeName

		err := vh.DeleteExchange(exchangeName)
		if err != nil {
			return nil, err
		}

		frame := b.framer.CreateExchangeDeleteFrame(request.Channel)

		if err := b.framer.SendFrame(conn, frame); err != nil {
			log.Error().Err(err).Msg("Failed to send exchange delete frame")
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported command")
	}
}
