package broker

import (
	"fmt"
	"net"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/rs/zerolog/log"
)

func (b *Broker) channelHandler(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	switch request.MethodID {
	case uint16(amqp.CHANNEL_OPEN):
		return b.handleOpenChannel(request, conn)
	case uint16(amqp.CHANNEL_FLOW):
		return b.handleChannelFlow(request, vh, conn)
	case uint16(amqp.CHANNEL_FLOW_OK):
		return b.handleChannelFlowOk(request, conn)
	case uint16(amqp.CHANNEL_CLOSE):
		return b.handleChannelClose(request, conn)
	case uint16(amqp.CHANNEL_CLOSE_OK):
		return b.handleChannelCloseOk(request, conn)
	default:
		log.Debug().Uint16("method_id", request.MethodID).Msg("Unknown channel method")
		return nil, fmt.Errorf("unknown channel method: %d", request.MethodID)
	}
}

// handleOpenChannel executes the AMQP command CHANNEL_OPEN
func (b *Broker) handleOpenChannel(request *amqp.RequestMethodMessage, conn net.Conn) (any, error) {
	log.Debug().Interface("request", request).Msg("Received channel open request")

	// Check if the channel is already open
	if b.checkChannel(conn, request.Channel) {
		log.Debug().Uint16("channel", request.Channel).Msg("Channel already open")
		return nil, fmt.Errorf("channel already open")
	}
	b.registerChannel(conn, request)
	log.Trace().Interface("state", b.Connections[conn].Channels[request.Channel]).Msg("New state added")

	frame := b.framer.CreateChannelOpenOkFrame(request.Channel)
	if err := b.framer.SendFrame(conn, frame); err != nil {
		log.Error().Err(err).Msg("Failed to send channel open ok frame")
	}
	return nil, nil
}

func (b *Broker) handleChannelFlow(request *amqp.RequestMethodMessage, vh *vhost.VHost, conn net.Conn) (any, error) {
	channel := request.Channel
	content, ok := request.Content.(*amqp.ChannelFlowContent)
	if !ok {
		log.Error().Uint16("channel", channel).Msg("Invalid content for channel flow")
		// Raise 501 error if content is invalid
		amqpErr := errors.NewConnectionError(
			"invalid content for channel flow",
			uint16(amqp.FRAME_ERROR),
			uint16(amqp.CHANNEL),
			uint16(amqp.CHANNEL_FLOW),
		)
		b.sendConnectionClosing(
			conn,
			channel,
			amqpErr.ReplyCode(),
			amqpErr.ClassID(),
			amqpErr.MethodID(),
			amqpErr.Error(),
		)
		return nil, amqpErr
	}
	flowActive := content.Active

	b.mu.Lock()
	_, exists := b.Connections[conn].Channels[channel]
	b.mu.Unlock()
	if !exists {
		// Client tried to use a non-registered channel -- it should send a channel.open first
		// Architecture decision: enforce that client must open the channel first
		log.Debug().Uint16("channel", channel).Msg("Channel not open")
		amqpErr := errors.NewConnectionError(
			fmt.Sprintf("channel '%d' not found in vhost '%s'", channel, vh.Name),
			uint16(amqp.CHANNEL_ERROR),
			uint16(amqp.CHANNEL),
			uint16(amqp.CHANNEL_FLOW),
		)
		b.sendChannelClosing(
			conn,
			channel,
			amqpErr.ReplyCode(),
			amqpErr.ClassID(),
			amqpErr.MethodID(),
			amqpErr.Error(),
		)
		return nil, amqpErr
	}
	// Client-initiated flow control (flowInitiatedByBroker = false)
	// This allows client to request flow pause/resume for delivery throttling
	// Architecture decision: always honor flow change requested by client
	// Further improvements may include broker policies to override client requests
	flowInitiatedByBroker := false
	connID, ok := b.GetConnectionID(conn)
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}
	err := vh.HandleChannelFlow(connID, channel, flowActive, flowInitiatedByBroker)
	if err != nil {
		log.Error().Err(err).Msg("Failed to handle channel flow in vhost")
		// Should raise 541 error if vhost fails to handle flow
		amqpErr := errors.NewConnectionError(
			fmt.Sprintf("vhost '%s' failed to handle channel flow", vh.Name),
			uint16(amqp.INTERNAL_ERROR),
			uint16(amqp.CHANNEL),
			uint16(amqp.CHANNEL_FLOW),
		)
		b.sendChannelClosing(
			conn,
			channel,
			amqpErr.ReplyCode(),
			amqpErr.ClassID(),
			amqpErr.MethodID(),
			amqpErr.Error(),
		)
		return nil, amqpErr
	}

	log.Debug().Uint16("channel", channel).Bool("flow_active", flowActive).Msg("Channel flow state updated")
	// send channel.flow-ok
	frame := b.framer.CreateChannelFlowOkFrame(channel, flowActive)
	err = b.framer.SendFrame(conn, frame)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send channel flow ok frame")
		return nil, err
	}

	return nil, nil
}

// handleChannelFlowOk executes the AMQP command CHANNEL_FLOW_OK. This is just an acknowledgment from the client.
// Since the broker does not currently initiate flow control, this method just verifies if the channel exists.
func (b *Broker) handleChannelFlowOk(request *amqp.RequestMethodMessage, conn net.Conn) (any, error) {
	channel := request.Channel
	b.mu.Lock()
	_, exists := b.Connections[conn].Channels[channel]
	b.mu.Unlock()
	if !exists {
		log.Debug().Uint16("channel", channel).Msg("Channel not found for flow ok")
		// raise 504 error if channel not found
		amqpErr := errors.NewConnectionError(
			fmt.Sprintf("channel '%d' not found", channel),
			uint16(amqp.CHANNEL_ERROR),
			uint16(amqp.CHANNEL),
			uint16(amqp.CHANNEL_FLOW_OK),
		)
		b.sendChannelClosing(
			conn,
			channel,
			amqpErr.ReplyCode(),
			amqpErr.ClassID(),
			amqpErr.MethodID(),
			amqpErr.Error(),
		)
		return nil, amqpErr
	}
	log.Debug().Uint16("channel", channel).Msg("Channel flow ok received")
	return nil, nil
}

func (b *Broker) handleChannelClose(request *amqp.RequestMethodMessage, conn net.Conn) (any, error) {
	b.mu.Lock()
	channelState, exists := b.Connections[conn].Channels[request.Channel]
	if !exists {
		log.Debug().Uint16("channel", request.Channel).Msg("Channel already closed") // no need to rise an error here
		return nil, nil
	}
	b.mu.Unlock()
	// send channel close ok
	frame := b.framer.CreateChannelCloseOkFrame(request.Channel)
	err := b.framer.SendFrame(conn, frame)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send channel close ok frame")
		return nil, err
	}
	b.mu.Lock()
	channelState.ClosingChannel = true
	b.mu.Unlock()
	return nil, nil
}

func (b *Broker) handleChannelCloseOk(request *amqp.RequestMethodMessage, conn net.Conn) (any, error) {
	b.removeChannel(conn, request.Channel)
	return nil, nil
}

// registerChannel register a new channel to the connection
func (b *Broker) registerChannel(conn net.Conn, frame *amqp.RequestMethodMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Connections[conn].Channels[frame.Channel] = &amqp.ChannelState{MethodFrame: frame}
	log.Debug().Uint16("channel", frame.Channel).Msg("New channel added")
}

// removeChannel removes a channel from the connection
func (b *Broker) removeChannel(conn net.Conn, channel uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.Connections[conn].Channels, channel)
}

// checkChannel checks if a channel is already open
func (b *Broker) checkChannel(conn net.Conn, channel uint16) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.Connections[conn].Channels[channel]
	return ok
}
