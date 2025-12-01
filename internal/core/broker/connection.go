package broker

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/amqp/errors"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/rs/zerolog/log"
)

func (b *Broker) handleConnection(conn net.Conn, connInfo *amqp.ConnectionInfo) {
	b.ActiveConns.Add(1)
	client := connInfo.Client
	// ctx := client.Ctx

	defer func() {
		defer b.ActiveConns.Done()
		if len(b.Connections) == 0 {
			log.Debug().Msg("No connections to clean")
			return
		}
		log.Debug().Msg("Cleaning connection")
		b.cleanupConnection(conn)
	}()

	channelNum := uint16(0)

	for {
		frame, err := b.framer.ReadFrame(conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Debug().Err(err).Msg("Connection timeout")
			}
			if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
				log.Debug().Str("client", conn.RemoteAddr().String()).Msg("Connection closed by client")
			} else {
				log.Error().Err(err).Msg("Error reading frame")
			}
			client.Cancel()
			return
		}

		if len(frame) > 0 { // any octet shall be valid as heartbeat #AMQP_compliance
			b.registerHeartbeat(conn)
		}

		//Process frame
		newInterface, err := b.framer.ParseFrame(frame)
		if err != nil {
			log.Error().Err(err).Msg("Failed parsing frame")
			client.Cancel()
			return
		}
		if _, ok := newInterface.(*amqp.Heartbeat); ok {
			continue
		}

		newState, ok := newInterface.(*amqp.ChannelState)
		if !ok {
			log.Error().Msg("Failed to cast request to ChannelState")
			client.Cancel()
			return
		}

		log.Trace().Interface("state", newState).Msg("New State")

		if newState.MethodFrame != nil {
			request := newState.MethodFrame
			if channelNum != request.Channel {
				channelNum = newState.MethodFrame.Channel
			}
		} else {
			if newState.HeaderFrame != nil {
				log.Trace().Interface("header", newState.HeaderFrame).Msg("HeaderFrame")
			} else if newState.Body != nil {
				log.Trace().Interface("body", newState.Body).Msg("Body")
			}
			if previousState, exists := b.Connections[conn].Channels[channelNum]; exists {
				newState.MethodFrame = previousState.MethodFrame
				log.Trace().Interface("method_frame", previousState.MethodFrame).Msg("Recovered method frame")
			} else {
				log.Trace().Uint16("channel", channelNum).Msg("Channel not found")
				continue
			}
		}
		if _, err := b.processRequest(conn, newState); err != nil {
			// if the error reaches here, it means that the error is at connection level and unexpected
			// Expected errors should be handled at method level and send channel.close instead
			if _, isChannelError := err.(*errors.ChannelError); isChannelError {
				log.Error().Err(err).Msg("Unexpected channel error at connection level")
			} else if _, isMethodError := err.(*errors.ConnectionError); isMethodError {
				log.Error().Err(err).Msg("Failed to process request")
			}
			request := newState.MethodFrame

			b.sendConnectionClosing(conn,
				request.Channel,
				uint16(amqp.INTERNAL_ERROR),
				uint16(request.ClassID),
				uint16(request.MethodID),
				err.Error(),
			)
			b.setConnectionClosingState(conn)
		}
	}
}

func (b *Broker) registerConnection(conn net.Conn, connInfo *amqp.ConnectionInfo) {
	b.mu.Lock()
	b.Connections[conn] = connInfo
	b.mu.Unlock()

	// Register connection ID in bidirectional maps
	connID := vhost.ConnectionID(GenerateConnectionID(conn))
	b.connectionsMu.Lock()
	b.connections[connID] = conn
	b.connToID[conn] = connID
	b.connectionsMu.Unlock()
}

func (b *Broker) cleanupConnection(conn net.Conn) {
	b.mu.Lock()
	connInfo, ok := b.Connections[conn]
	if !ok {
		b.mu.Unlock()
		log.Debug().Msg("cleanupConnection: connection not found in b.Connections")
		return
	}
	vhName := connInfo.VHostName
	delete(b.Connections, conn)
	b.mu.Unlock()

	// Remove from bidirectional maps
	b.connectionsMu.Lock()
	connID, hasConnID := b.connToID[conn]
	delete(b.connections, connID)
	delete(b.connToID, conn)
	b.connectionsMu.Unlock()

	connInfo.Client.Ctx.Done()
	vh := b.GetVHost(vhName)
	if vh != nil && hasConnID {
		vh.CleanupConnection(connID)
		log.Debug().Str("vhost", vhName).Str("conn_id", string(connID)).Msg("Cleaned up connection consumers and channels from vhost")
	}
}

// closeConnectionRequested closes a connection and sends a CONNECTION_CLOSE_OK frame
func (b *Broker) closeConnectionRequested(request *amqp.RequestMethodMessage, conn net.Conn) (any, error) {
	frame := b.framer.CreateConnectionCloseOkFrame(request.Channel)
	err := b.framer.SendFrame(conn, frame)
	b.mu.Lock()
	connInfo, exists := b.Connections[conn]
	if !exists {
		b.mu.Unlock()
		return nil, nil
	}
	connInfo.ClosingConnection = true
	b.mu.Unlock()
	return nil, err
}

func (b *Broker) isConnectionClosing(conn net.Conn) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	connInfo, exists := b.Connections[conn]
	if !exists {
		return false, fmt.Errorf("connection not found")
	}
	return connInfo.ClosingConnection, nil
}

// sendConnectionClosing sends `connection.close` when the server needs to shutdown for some reason
func (b *Broker) sendConnectionClosing(conn net.Conn, channel, replyCode, methodId, classId uint16, replyText string) (any, error) {
	frame := b.framer.CreateConnectionCloseFrame(channel, replyCode, methodId, classId, replyText)
	err := b.framer.SendFrame(conn, frame)

	return nil, err
}

func (b *Broker) connectionCloseOk(conn net.Conn) {
	b.cleanupConnection(conn)
	conn.Close()
}

// registerHeartbeat registers a heartbeat for a connection
func (b *Broker) registerHeartbeat(conn net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if connInfo, exists := b.Connections[conn]; exists {
		connInfo.Client.LastHeartbeat = time.Now()
	} else {
		// it means that the connection was closed
		// verify if the connection is still alive
		if _, err := conn.Write([]byte{}); err != nil {
			log.Debug().Err(err).Msg("Connection seems to be closed, cleaning up")
			b.cleanupConnection(conn)
		}
	}
}

func (b *Broker) BroadcastConnectionClose() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for conn := range b.Connections {
		if _, err := b.sendConnectionClosing(conn, 0, uint16(amqp.CONNECTION_FORCED), 0, 0, amqp.ReplyText[amqp.CONNECTION_FORCED]); err != nil {
			log.Error().Err(err).Msg("Failed to send close connection")
		}
	}
}

func (b *Broker) connectionHandler(request *amqp.RequestMethodMessage, conn net.Conn) (any, error) {
	switch request.MethodID {
	case uint16(amqp.CONNECTION_CLOSE):
		return b.closeConnectionRequested(request, conn)
	case uint16(amqp.CONNECTION_CLOSE_OK):
		b.connectionCloseOk(conn)
		return nil, nil
	default:
		log.Debug().Uint16("method_id", request.MethodID).Msg("Unknown connection method")
		return nil, fmt.Errorf("unknown connection method: %d", request.MethodID)
	}
}

// sendChannelClosing sends `channel.close` when the server raises a channel exception or needs to close the channel for some reason
func (b *Broker) sendChannelClosing(conn net.Conn, channel, replyCode, classId, methodId uint16, replyText string) error {
	b.mu.Lock()
	connInfo, exists := b.Connections[conn]
	if !exists {
		b.mu.Unlock()
		return fmt.Errorf("connection not found")
	}
	chState, exists := connInfo.Channels[channel]
	if !exists {
		b.mu.Unlock()
		return fmt.Errorf("channel %d not found", channel)
	}
	chState.ClosingChannel = true
	b.mu.Unlock()
	frame := b.framer.CreateChannelCloseFrame(channel, replyCode, classId, methodId, replyText)
	return b.framer.SendFrame(conn, frame)
}
