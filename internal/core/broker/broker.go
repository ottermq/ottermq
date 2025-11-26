package broker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andrelcunha/ottermq/config"
	"github.com/andrelcunha/ottermq/internal/core/amqp"
	"github.com/andrelcunha/ottermq/internal/core/broker/management"
	"github.com/andrelcunha/ottermq/internal/core/broker/vhost"
	"github.com/andrelcunha/ottermq/internal/core/models"
	"github.com/andrelcunha/ottermq/pkg/persistence"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/json"
	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/memento"
	"github.com/rs/zerolog/log"

	_ "github.com/andrelcunha/ottermq/internal/persistdb"
)

const (
	PLATFORM         = "golang"
	PRODUCT          = "OtterMQ"
	DEFAULT_PROTOCOL = "AMQP 0-9-1"
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
	startedAt     time.Time
}

func NewBroker(config *config.Config, rootCtx context.Context, rootCancel context.CancelFunc) *Broker {
	// Create persistence layer based on config
	persistConfig := &persistence.Config{
		Type:    "json", // from config or env var
		DataDir: config.DataDir,
		Options: make(map[string]string),
	}
	persist, err := json.NewJsonPersistence(persistConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create JSON persistence")
	}

	b := &Broker{
		VHosts:      make(map[string]*vhost.VHost),
		Connections: make(map[net.Conn]*amqp.ConnectionInfo),
		connections: make(map[vhost.ConnectionID]net.Conn),
		connToID:    make(map[net.Conn]vhost.ConnectionID),
		config:      config,
		rootCtx:     rootCtx,
		rootCancel:  rootCancel,
		persist:     persist,
		Ready:       make(chan struct{}),
		startedAt:   time.Now(),
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
		// TODO: dynamically set capabilities based on enabled features
		"basic.nack":             true,
		"connection.blocked":     true,
		"consumer_cancel_notify": true,
		"publisher_confirms":     true,
	}

	serverProperties := map[string]any{
		"capabilities": capabilities,
		"product":      PRODUCT,
		"version":      b.config.Version,
		"platform":     PLATFORM,
	}

	configurations := map[string]any{
		"mechanisms":        []string{"PLAIN"},
		"locales":           []string{"en_US"},
		"serverProperties":  serverProperties,
		"heartbeatInterval": b.config.HeartbeatIntervalMax,
		"frameMax":          b.config.FrameMax,
		"channelMax":        b.config.ChannelMax,
		"ssl":               b.config.Ssl,
		"protocol":          DEFAULT_PROTOCOL,
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

// ListConnections returns all active connections in the broker.
func (b *Broker) ListConnections() []amqp.ConnectionInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	connections := make([]amqp.ConnectionInfo, 0, len(b.Connections))
	for _, c := range b.Connections {
		connections = append(connections, *c)
	}
	return connections
}

// listConnectionsPerVhosts returns a map of vhost names to their respective connections.
func (b *Broker) listConnectionsPerVhosts() map[string][]amqp.ConnectionInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	vhostConnections := make(map[string][]amqp.ConnectionInfo)
	for _, c := range b.Connections {
		vhostName := c.VHostName
		vhostConnections[vhostName] = append(vhostConnections[vhostName], *c)
	}
	return vhostConnections
}

func (b *Broker) GetConnectionByName(name string) (*amqp.ConnectionInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.Connections {
		if string(b.connToID[c.Client.Conn]) == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("connection '%s' not found", name)
}

func (b *Broker) Shutdown() {
	for conn := range b.Connections {
		conn.Close()
	}
}

// ListChannels returns all channels across all connections.
func (b *Broker) ListChannels() ([]models.ChannelInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	channels := make([]models.ChannelInfo, 0, len(b.Connections))
	for conn, c := range b.Connections {
		channels = append(channels, b.listChannelsByConnection(c, conn)...)
	}

	return channels, nil
}

func (b *Broker) ListConnectionChannels(connName string) ([]models.ChannelInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for conn, c := range b.Connections {
		if connName == string(b.connToID[conn]) {
			return b.listChannelsByConnection(c, conn), nil
		}
	}
	return nil, fmt.Errorf("connection '%s' not found", connName)
}

// ListConnectionChannels returns all channels for a specific connection.
func (b *Broker) listChannelsByConnection(connInfo *amqp.ConnectionInfo, conn net.Conn) []models.ChannelInfo {
	channels := make([]models.ChannelInfo, 0, len(b.Connections))
	vhostName := connInfo.VHostName
	vh := b.VHosts[vhostName]
	user := connInfo.Client.Config.Username
	connID := b.connToID[conn]
	for channelNum, ch := range connInfo.Channels {
		channelInfo, err := b.createChannelInfo(connID, channelNum, vh, ch, user)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to create channel summary")
			continue
		}
		channels = append(channels, channelInfo)
	}
	return channels
}

func (b *Broker) CreateChannelInfo(connID vhost.ConnectionID, channelNum uint16, vh *vhost.VHost) (models.ChannelInfo, error) {
	b.mu.Lock()
	connInfo := b.Connections[b.connections[connID]]
	ch := connInfo.Channels[channelNum]
	user := connInfo.Client.Config.Username
	b.mu.Unlock()
	return b.createChannelInfo(connID, channelNum, vh, ch, user)
}

// createChannelInfo creates a summary of the channel state.
func (*Broker) createChannelInfo(connID vhost.ConnectionID, channelNum uint16, vh *vhost.VHost, ch *amqp.ChannelState, user string) (models.ChannelInfo, error) {
	if connID == "" {
		return models.ChannelInfo{}, fmt.Errorf("connection ID is empty")
	}

	if vh == nil {
		return models.ChannelInfo{}, fmt.Errorf("vhost is nil")
	}

	if ch == nil {
		return models.ChannelInfo{}, fmt.Errorf("channel state is nil")
	}

	channelKey := vhost.ConnectionChannelKey{
		ConnectionID: connID,
		Channel:      channelNum,
	}

	deliveryState := vh.ChannelDeliveries[channelKey]
	inTransaction := false
	txState := vh.GetTransactionState(channelNum, connID)
	if txState != nil {
		txState.Lock()
		inTransaction = txState.InTransaction
		txState.Unlock()
	}
	channelStateLabel := getChannelState(deliveryState, ch)
	channelInfo := models.ChannelInfo{
		VHost:            vh.Name,
		Number:           channelNum,
		ConnectionName:   string(connID),
		User:             user,
		State:            channelStateLabel,
		UnconfirmedCount: 0, // Relevant only when publisher confirms mode is enabled. Not tracked yet.
		UnackedCount:     len(deliveryState.UnackedByTag),
		PrefetchCount:    deliveryState.NextPrefetchCount,
		InTransaction:    inTransaction,
		ConfirmMode:      false, // Not implemented yet
	}
	return channelInfo, nil
}

// getChannelState returns the state of the channel as a string ("running" or "flow").
func getChannelState(deliveryState *vhost.ChannelDeliveryState, channelState *amqp.ChannelState) string {
	if channelState != nil && channelState.ClosingChannel {
		return "closing"
	}
	if deliveryState != nil && !deliveryState.FlowActive {
		return "flow"
	}
	return "running"
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

func (b *Broker) GetBrokerOverviewConfig() models.BrokerConfigOverview {
	b.mu.Lock()
	defer b.mu.Unlock()

	enabledFeatures := b.getEnabledFeatures()
	ctx := b.getContexts()

	persistence := b.getPersistenceBackend()

	return models.BrokerConfigOverview{
		AMQPPort:           b.config.BrokerPort,
		SSL:                b.config.Ssl, // this one refers to AMQP SSL/TLS
		HTTPPort:           b.config.WebPort,
		EnabledFeatures:    enabledFeatures,
		ChannelMax:         int(b.config.ChannelMax),
		FrameMax:           int(b.config.FrameMax),
		QueueBufferSize:    b.config.QueueBufferSize,
		PersistenceBackend: persistence,
		HTTPContexts:       ctx,
	}
}

// getPersistenceBackend returns the name of the persistence backend in use.
func (b *Broker) getPersistenceBackend() string {
	persistence := "unknown"
	switch b.persist.(type) {
	case *json.JsonPersistence:
		persistence = "json"
	case *dummy.DummyPersistence:
		persistence = "dummy"
	case *memento.MementoPersistence:
		persistence = "memento"
	}
	return persistence
}

// getContexts returns the list of HTTP contexts (e.g., management UI, prometheus) available in the broker.
func (b *Broker) getContexts() []models.HttpContext {
	ctx := make([]models.HttpContext, 0)
	if b.config.EnableWebAPI && b.config.EnableUI {
		ctx = append(ctx, models.HttpContext{
			Name: "management",
			Port: b.config.WebPort,
			Path: b.config.WebAPIPath,
		})
	}
	// Prometheus context in the future
	return ctx
}

// getEnabledPlugins returns the list of enabled plugins in the node.
func (b *Broker) getEnabledPlugins() []string {
	enabledPlugins := []string{}
	if b.config.EnableWebAPI {
		enabledPlugins = append(enabledPlugins, "WebAPI")

	}
	if b.config.EnableSwagger {
		enabledPlugins = append(enabledPlugins, "Swagger")

	}
	if b.config.EnableUI {
		enabledPlugins = append(enabledPlugins, "WebUI")

	}
	return enabledPlugins
}

// getEnabledFeatures returns the list of enabled features in the node.
func (b *Broker) getEnabledFeatures() []string {
	enabledFeatures := []string{}
	if b.config.EnableDLX {
		enabledFeatures = append(enabledFeatures, "DLX")
	}
	if b.config.EnableTTL {
		enabledFeatures = append(enabledFeatures, "TTL")
	}
	if b.config.EnableQLL {
		enabledFeatures = append(enabledFeatures, "QLL")
	}
	return enabledFeatures
}

// GetOverviewNodeDetails returns detailed information about the node.
func (b *Broker) GetOverviewNodeDetails() models.OverviewNodeDetails {
	b.mu.Lock()
	defer b.mu.Unlock()

	nodeName := fmt.Sprintf("ottermq@%s", getHostname())

	node := models.OverviewNodeDetails{
		Name:            nodeName,
		FDUsed:          int(getFileDescriptors()),
		FDLimit:         int(getFileDescriptorLimit()),
		Goroutines:      runtime.NumGoroutine(),
		GoroutinesLimit: 0, // No hard limit in Go, TBD if we should set one
		MemoryUsage:     int(getMemoryUsage()),
		Cores:           runtime.NumCPU(),
		// Info: models.NodeInfo{
	}
	sysInfo, err := getSysInfo()
	if err == nil {
		node.MemoryLimit = int(sysInfo.TotalRam)
		node.UptimeSecs = int(sysInfo.Uptime)
		node.DiskTotal = int(sysInfo.TotalDisk)
		node.DiskAvailable = int(sysInfo.AvailDisk)
	}

	node.Info = models.NodeInfo{
		MessageRates:   "basic", // default strategy
		EnabledPlugins: b.getEnabledPlugins(),
	}
	return node
}

func (b *Broker) GetOverviewConnStats() models.OverviewConnectionStats {
	b.mu.Lock()
	defer b.mu.Unlock()

	var stats models.OverviewConnectionStats
	for _, connInfo := range b.Connections {
		stats.Total++
		if connInfo.ClosingConnection {
			stats.Closing++
		} else {
			stats.Running++
		}
		if connInfo.Client.Config.Protocol == DEFAULT_PROTOCOL {
			stats.AMQP091++
		}
	}
	channelKey := vhost.ConnectionChannelKey{
		ConnectionID: vhost.MANAGEMENT_CONNECTION_ID,
		Channel:      0,
	}
	for _, vh := range b.VHosts {
		ch := vh.GetChannelDelivery(channelKey)
		if ch != nil {
			stats.Total++   // each management connection has one channel (0) on each vhost
			stats.Running++ // management channels are always running
		}
	}

	stats.ClientConnections = stats.Total // In the future, exclude management connections

	return stats
}

// GetBrokerOverviewDetails returns detailed information about the broker.
func (b *Broker) GetBrokerOverviewDetails() models.OverviewBrokerDetails {
	b.mu.Lock()
	defer b.mu.Unlock()
	build := getCommitInfo(b.config.Version)
	return models.OverviewBrokerDetails{
		Product:    PRODUCT,
		Version:    build.Version,
		CommitInfo: build,
		Platform:   PLATFORM,
		GoVersion:  runtime.Version(),
		UptimeSecs: int(time.Since(b.startedAt).Seconds()),
		StartTime:  b.startedAt.Format(time.RFC3339),
		DataDir:    b.config.DataDir,
	}
}

func (b *Broker) GetObjectTotalsOverview() models.OverviewObjectTotals {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Count objects across all vhosts
	var totals models.OverviewObjectTotals
	vhostConnections := b.listConnectionsPerVhosts()
	for _, vh := range b.VHosts {
		totals.Connections += len(vhostConnections[vh.Name])
		totals.Channels += b.getChannelCountByVHost(vh)
		totals.Exchanges += b.getExchangeCountByVHost(vh)
		totals.Queues += b.getQueueCountByVHost(vh)
		totals.Consumers += b.getConsumersByVHost(vh)
	}
	return totals
}

func (b *Broker) getChannelCountByVHost(vh *vhost.VHost) int {
	channels, err := b.ListConnectionChannels(vh.Name)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to list channels for vhost")
	}
	return len(channels)
}

func (b *Broker) getExchangeCountByVHost(vh *vhost.VHost) int {
	return len(vh.GetAllExchanges())
}

func (b *Broker) getQueueCountByVHost(vh *vhost.VHost) int {
	return len(vh.GetAllQueues())
}

func (b *Broker) getConsumersByVHost(vh *vhost.VHost) int {
	return len(vh.GetAllConsumers())
}
