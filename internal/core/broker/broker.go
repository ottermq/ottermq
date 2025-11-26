package broker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/andrelcunha/ottermq/config"
	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/management"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/pkg/persistence"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/json"
	"github.com/rs/zerolog/log"

	_ "github.com/andrelcunha/ottermq/internal/persistdb"
)

const (
	platform = "golang"
	product  = "OtterMQ"
)

type Broker struct {
	VHosts       map[string]*vhost.VHost
	config       *config.Config                    `json:"-"`
	listener     net.Listener                      `json:"-"`
	Connections  map[net.Conn]*amqp.ConnectionInfo `json:"-"`
	mu           sync.Mutex                        `json:"-"`
	framer       amqp.Framer
	ShuttingDown atomic.Bool
	ActiveConns  sync.WaitGroup
	rootCtx      context.Context
	rootCancel   context.CancelFunc
	persist      persistence.Persistence
	Ready        chan struct{} // Signals when the broker is ready to accept connections
	Management   management.ManagementService

	connections   map[vhost.ConnectionID]net.Conn
	connToID      map[net.Conn]vhost.ConnectionID // Reverse map
	connectionsMu sync.RWMutex
}

func NewBroker(config *config.Config, rootCtx context.Context, rootCancel context.CancelFunc) *Broker {
	// Create persistence layer based on config
	persistConfig := &persistence.Config{
		Type:    "json", // from config or env var
		DataDir: "data",
		Options: make(map[string]string),
	}
	persist, err := json.NewJsonPersistence(persistConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create JSON persistence")
	}

	b := &Broker{
		VHosts:      make(map[string]*vhost.VHost),
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		config:      config,
		rootCtx:     rootCtx,
		rootCancel:  rootCancel,
		persist:     persist,
		Ready:       make(chan struct{}),
	}
	options := vhost.VHostOptions{
		QueueBufferSize: config.QueueBufferSize,
		Persistence:     b.persist,
		EnableDLX:       config.EnableDLX,
		EnableTTL:       config.EnableTTL,
		EnableQLL:       config.EnableQLL,
	}

	defaultVHost := vhost.NewVhost("/", options)
	b.VHosts["/"] = defaultVHost

	b.framer = &amqp.DefaultFramer{}
	b.VHosts["/"].SetFramer(b.framer)

	defaultVHost.SetFrameSender(b)

	b.Management = management.NewService(b)
	return b
}

func (b *Broker) Start() error {
	if b.config.ShowLogo {
		Logo()
	}
	log.Info().Str("version", b.config.Version).Msg("🦦 OtterMQ")

	configurations := b.setConfigurations()

	addr := fmt.Sprintf("%s:%s", b.config.BrokerHost, b.config.BrokerPort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start listener")
	}
	b.listener = listener
	defer listener.Close()
	log.Info().Str("addr", addr).Msg("Started TCP listener")

	// Signal that the broker is ready to accept connections
	close(b.Ready)

	return b.acceptLoop(configurations)
}

// Logo 🦦
func Logo() {
	fmt.Print(`
	
 .d88888b.  888   888                  888b     d888 .d88888b. 
d88P" "Y88b 888   888                  8888b   d8888d88P" "Y88b
888     888 888   888                  88888b.d88888888     888
888     888 888888888888 .d88b. 888d888888Y88888P888888     888
888     888 888   888   d8P  Y8b888P"  888 Y888P 888888     888
888     888 888   888   88888888888    888  Y8P  888888 Y8b 888
Y88b. .d88P Y88b. Y88b. Y8b.    888    888   "   888Y88b.Y8b88P
 "Y88888P"   "Y888 "Y888 "Y8888 888    888       888 "Y888888" 
                                                          Y8b

`)
}

func (b *Broker) setConfigurations() map[string]any {
	capabilities := map[string]any{
		"basic.nack":             true,
		"connection.blocked":     true,
		"consumer_cancel_notify": true,
		"publisher_confirms":     true,
	}

	serverProperties := map[string]any{
		"capabilities": capabilities,
		"product":      product,
		"version":      b.config.Version,
		"platform":     platform,
	}

	configurations := map[string]any{
		"mechanisms":        []string{"PLAIN"},
		"locales":           []string{"en_US"},
		"serverProperties":  serverProperties,
		"heartbeatInterval": b.config.HeartbeatIntervalMax,
		"frameMax":          b.config.FrameMax,
		"channelMax":        b.config.ChannelMax,
		"ssl":               b.config.Ssl,
		"protocol":          "AMQP 0-9-1",
	}
	return configurations
}

func (b *Broker) acceptLoop(configurations map[string]any) error {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || b.rootCtx.Err() != nil {
				log.Info().Err(err).Msg("Listener closed or context canceled")
				return err
			}
			log.Debug().Err(err).Msg("Accept failed")
			continue
		}
		if ok := b.ShuttingDown.Load(); ok {
			log.Debug().Msg("Broker is shutting down, ignoring new connection")
			continue
		}
		log.Debug().Str("client", conn.RemoteAddr().String()).Msg("New client waiting for connection")
		connCtx, connCancel := context.WithCancel(b.rootCtx)
		defer connCancel()
		connInfo, err := b.framer.Handshake(&configurations, conn, connCtx)
		if err != nil {
			log.Info().Err(err).Msg("Handshake failed")
			continue
		}
		b.registerConnection(conn, connInfo)
		go b.monitorConnectionLifecycle(conn, connInfo.Client)

		go b.handleConnection(conn, connInfo)
	}
}

func (b *Broker) monitorConnectionLifecycle(conn net.Conn, client *amqp.AmqpClient) {
	<-client.Ctx.Done()
	log.Info().Str("client", conn.RemoteAddr().String()).Msg("Connection closed")
	b.cleanupConnection(conn)
}

func (b *Broker) processRequest(conn net.Conn, newState *amqp.ChannelState) (any, error) {
	request := newState.MethodFrame
	connInfo, exist := b.Connections[conn]
	if !exist {
		// connection terminated while processing the request
		log.Info().Str("client", conn.RemoteAddr().String()).Msg("Connection closed")
		return nil, nil
	}
	vh := b.VHosts[connInfo.VHostName]
	isConnectionClosing, err := b.isConnectionClosing(conn)
	if err != nil {
		return nil, err
	}
	isShuttingDown := b.ShuttingDown.Load()
	if isConnectionClosing || isShuttingDown {
		log.Debug().Str("client", conn.RemoteAddr().String()).Msg("Connection is closing or broker is shutting down, ignoring further requests")
		if request.ClassID != uint16(amqp.CONNECTION) ||
			(request.MethodID != uint16(amqp.CONNECTION_CLOSE) &&
				request.MethodID != uint16(amqp.CONNECTION_CLOSE_OK)) {
			return nil, nil
		}
	}
	// Check if the channel is in closing state (stored state, not incoming frame)
	if request.Channel > 0 {
		b.mu.Lock()
		if storedState, exists := connInfo.Channels[request.Channel]; exists && storedState.ClosingChannel {
			b.mu.Unlock()
			// Allow channel.close and channel.close-ok through, but ignore all other methods
			if request.ClassID != uint16(amqp.CHANNEL) ||
				(request.MethodID != uint16(amqp.CHANNEL_CLOSE) &&
					request.MethodID != uint16(amqp.CHANNEL_CLOSE_OK)) {
				log.Debug().Uint16("channel", request.Channel).Msg("Channel closing - ignoring non-close request")
				return nil, nil
			}
		} else {
			b.mu.Unlock()
		}
	}

	switch request.ClassID {
	case uint16(amqp.CONNECTION):
		return b.connectionHandler(request, conn)
	case uint16(amqp.CHANNEL):
		return b.channelHandler(request, vh, conn)
	case uint16(amqp.EXCHANGE):
		return b.exchangeHandler(request, vh, conn)
	case uint16(amqp.QUEUE):
		return b.queueHandler(request, vh, conn)
	case uint16(amqp.BASIC):
		return b.basicHandler(newState, vh, conn)
	case uint16(amqp.TX):
		return b.txHandler(request, vh, conn)
	default:
		return nil, fmt.Errorf("unsupported command")
	}
}

func (b *Broker) getCurrentState(conn net.Conn, channel uint16) *amqp.ChannelState {
	b.mu.Lock()
	defer b.mu.Unlock()
	state, ok := b.Connections[conn].Channels[channel]
	if !ok {
		log.Debug().Uint16("channel", channel).Msg("No channel found")
		return nil
	}
	return state
}

func (b *Broker) GetVHost(vhostName string) *vhost.VHost {
	b.mu.Lock()
	defer b.mu.Unlock()
	if vhost, ok := b.VHosts[vhostName]; ok {
		return vhost
	}
	return nil
}

func (b *Broker) ListVHosts() []*vhost.VHost {
	b.mu.Lock()
	defer b.mu.Unlock()
	vhosts := make([]*vhost.VHost, 0, len(b.VHosts))
	for _, vh := range b.VHosts {
		vhosts = append(vhosts, vh)
	}
	return vhosts
}

func (b *Broker) ListConnections() []amqp.ConnectionInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	connections := make([]amqp.ConnectionInfo, 0, len(b.Connections))
	for _, c := range b.Connections {
		connections = append(connections, *c)
	}
	return connections
}

func (b *Broker) Shutdown() {
	for conn := range b.Connections {
		conn.Close()
	}
}

func (b *Broker) ListChannels() ([]ChannelInformation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	channels := make([]ChannelInformation, 0, len(b.Connections))
	for _, c := range b.Connections {
		vhost := c.VHostName
		vh := b.VHosts[vhost]
		user := c.Client.Config.Username
		for ck, consumer := range vh.Consumers {

			channelInfo := ChannelInformation{
				VHost:          vhost,
				Number:         ck.Channel,
				ConnectionName: string(consumer.ConnectionID),
				User:           user,
				State:          "running",
				// UnconfirmedCount:   ch.UnconfirmedCount,
				// UnackedCount:      ch.UnackedCount,
				// PrefetchCount:      ch.PrefetchCount,
				// InTransaction:      ch.InTransaction,
				// ConfirmMode:        ch.ConfirmMode,
			}
			channels = append(channels, channelInfo)
		}

	}

	return channels, nil
}

type ChannelInformation struct {
	Number           uint16 `json:"number"`
	ConnectionName   string `json:"connection_name"`
	VHost            string `json:"vhost"`
	User             string `json:"user"`
	State            string `json:"state"` // "running"
	UnconfirmedCount int    `json:"unconfirmed_count"`
	PrefetchCount    uint16 `json:"prefetch_count"`
	UnackedCount     int    `json:"unacked_count"`
}

func GenerateConnectionID(conn net.Conn) string {
	// connID := fmt.Sprintf("%s:%s", conn.RemoteAddr().String(), uuid.New().String()[:8])
	connID := fmt.Sprintf("%s", conn.RemoteAddr().String())
	return connID
}

// GetConnectionID returns the ConnectionID for a given net.Conn
func (b *Broker) GetConnectionID(conn net.Conn) (vhost.ConnectionID, bool) {
	b.connectionsMu.RLock()
	defer b.connectionsMu.RUnlock()
	connID, ok := b.connToID[conn]
	return connID, ok
}

func (b *Broker) SendFrame(connID vhost.ConnectionID, channelID uint16, frame []byte) error {
	b.connectionsMu.RLock()
	conn := b.connections[connID]
	b.connectionsMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection %s not found", connID)
	}

	return b.framer.SendFrame(conn, frame)
}
