package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/andrelcunha/ottermq/config"
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/persistdb"
	"github.com/andrelcunha/ottermq/pkg/logger"
	"github.com/andrelcunha/ottermq/web"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

var (
	VERSION = ""
)

// @title OtterMQ API
// @version 1.0
// @description API documentation for OtterMQ broker
// @host localhost:3000
// @BasePath /api/
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Load configuration from .env file, environment variables, or defaults
	cfg := config.LoadConfig(VERSION)

	// Initialize logger with configured log level
	logger.Init(cfg.LogLevel)

	// Determine the directory of the running binary
	dataDir := filepath.Join("data")

	// Ensure the data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		log.Info().Msg("Data directory not found. Creating a new one...")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Fatal().Err(err).Msg("Failed to create data directory")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := broker.NewBroker(cfg, ctx, cancel)

	// Verify if the database file exists
	log.Info().Msg("Searching for database...")
	dbPath := filepath.Join(dataDir, "ottermq.db")
	persistdb.SetDbPath(dbPath)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Info().Msg("Database file not found. Creating a new one...")
		persistdb.InitDB()
		persistdb.AddDefaultRoles()
		persistdb.AddDefaultPermissions()
		user := persistdb.UserCreateDTO{Username: cfg.Username, Password: cfg.Password, RoleID: 1}
		if err := persistdb.AddUser(user); err != nil {
			log.Error().Err(err).Msg("Failed to add user")
		}
		persistdb.CloseDB()
	}
	if err := persistdb.OpenDB(); err != nil {
		log.Error().Err(err).Msg("Failed to open database")
	}
	user, err := persistdb.GetUserByUsername(cfg.Username)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get user")
	}
	if user.RoleID != 1 {
		log.Fatal().Msg("User is not an admin")
	}
	persistdb.CloseDB()
	b.VHosts["/"].Users[user.Username] = &user

	// Start the broker in a goroutine
	go func() {
		err := b.Start()
		if err != nil {
			log.Fatal().Err(err).Msg("Broker error")
		}
	}()

	// Conditionally start web server based on EnableWebAPI flag
	var webServer *web.WebServer
	var app interface{ ShutdownWithContext(context.Context) error }
	var logfile *os.File

	if cfg.EnableWebAPI {
		log.Info().Msg("Web API enabled - initializing web server...")

		// Initialize the web admin server
		webConfig := &web.Config{
			BrokerHost:    cfg.BrokerHost,
			BrokerPort:    cfg.BrokerPort,
			Username:      cfg.Username,
			Password:      cfg.Password,
			JwtKey:        cfg.JwtSecret,
			WebServerPort: cfg.WebPort,
		}
		// initialize amqp client connection
		conn, err := web.GetBrokerClient(webConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to connect to broker")
		}
		defer conn.Close()

		webServer, err = web.NewWebServer(webConfig, b, conn)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create web server")
		}
		defer webServer.Close()

		// open "server.log" for appending
		logfile, err = os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to open log file")
		}
		defer logfile.Close()

		app = webServer.SetupApp(logfile)

		// Start the web admin server in a goroutine
		go func() {
			addr := fmt.Sprintf(":%s", cfg.WebPort)
			log.Info().Str("addr", addr).Msg("Starting web server")
			err := app.(*fiber.App).Listen(addr)
			if err != nil {
				log.Fatal().Err(err).Msg("Web server error")
			}
		}()
	} else {
		log.Info().Msg("Web API disabled - skipping web server initialization")
	}

	// Handle OS signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Info().Msg("Shutting down OtterMQ...")
	cancel()
	b.ShuttingDown.Store(true)

	// Broadcast connection close to all channels
	b.BroadcastConnectionClose()
	log.Info().Msg("Waiting for active connections to close...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		b.ActiveConns.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("All connections closed gracefully")
	case <-shutdownCtx.Done():
		log.Warn().Msg("Timeout reached. Forcing shutdown")
	}

	b.Shutdown()

	// Shutdown the web server if it was started
	if cfg.EnableWebAPI && app != nil {
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Fatal().Err(err).Msg("Failed to shutdown web server")
		}
		log.Info().Msg("Web server gracefully stopped")
	}
	log.Info().Msg("Server gracefully stopped")
	os.Exit(0) // if came so far it means the server has stopped gracefully
}
