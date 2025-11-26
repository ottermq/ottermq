package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andrelcunha/ottermq/config"
	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/internal/persistdb"
	"github.com/andrelcunha/ottermq/pkg/logger"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	testBroker     *broker.Broker
	testBrokerCtx  context.Context
	testBrokerStop context.CancelFunc
)

const brokerURL = "amqp://guest:guest@localhost:5672/"

// TestMain runs before all tests and sets up/tears down the broker
func TestMain(m *testing.M) {
	// Set up logger for tests (less verbose)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.DebugLevel) // Enable debug for DLX testing

	// Verify that no other broker is running on the test port
	conn, err := amqp.Dial(brokerURL)
	if err == nil {
		conn.Close()
		log.Debug().Msg("Another AMQP broker is running on the test port. Using existing broker for tests.")
	} else {

		// Start the broker for tests
		if err := setupBroker(); err != nil {
			log.Fatal().Err(err).Msg("Failed to setup broker for e2e tests")
		}

		// Wait for broker to be ready
		if err := waitForBroker(30 * time.Second); err != nil {
			testBrokerStop()
			log.Fatal().Err(err).Msg("Broker did not become ready in time")
		}
	}

	log.Info().Msg("Broker is ready for e2e tests")

	// Run all tests
	code := m.Run()

	// Tear down broker
	teardownBroker()

	os.Exit(code)
}

func setupBroker() error {
	// Create a temporary data directory for tests
	testDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("ottermq-test-%d", time.Now().Unix()))
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create test data directory: %w", err)
	}

	// Configure the broker for tests
	cfg := &config.Config{
		BrokerHost:      "localhost",
		BrokerPort:      "5672",
		Username:        "guest",
		Password:        "guest",
		LogLevel:        "warn",
		QueueBufferSize: 100000,
		JwtSecret:       "test-secret",
		WebPort:         "3001", // Different port to avoid conflicts
		DataDir:         testDataDir,
		EnableDLX:       true, // Enable DLX for tests
		EnableTTL:       true, // Enable TTL for tests
		EnableQLL:       true, // Enable Queue Length Limiter (QLL) for tests
	}

	logger.Init(cfg.LogLevel)

	// Setup database for tests
	dbPath := filepath.Join(testDataDir, "ottermq-test.db")
	persistdb.SetDbPath(dbPath)
	persistdb.InitDB()
	persistdb.AddDefaultRoles()
	persistdb.AddDefaultPermissions()
	user := persistdb.UserCreateDTO{
		Username: cfg.Username,
		Password: cfg.Password,
		RoleID:   1,
	}
	if err := persistdb.AddUser(user); err != nil {
		return fmt.Errorf("failed to add test user: %w", err)
	}
	persistdb.CloseDB()

	if err := persistdb.OpenDB(); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	dbUser, err := persistdb.GetUserByUsername(cfg.Username)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	persistdb.CloseDB()

	// Create broker
	testBrokerCtx, testBrokerStop = context.WithCancel(context.Background())
	testBroker = broker.NewBroker(cfg, testBrokerCtx, testBrokerStop)
	testBroker.VHosts["/"].Users[dbUser.Username] = &dbUser

	// Start broker in background
	go func() {
		if err := testBroker.Start(); err != nil {
			log.Error().Err(err).Msg("Test broker error")
		}
	}()

	return nil
}

func waitForBroker(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ticker.C:
			// Try to connect to the broker
			conn, err := amqp.Dial(brokerURL)
			if err == nil {
				conn.Close()
				return nil
			}
			// Continue waiting
		case <-testBrokerCtx.Done():
			return fmt.Errorf("broker context cancelled")
		}
	}

	return fmt.Errorf("timeout waiting for broker to be ready")
}

func teardownBroker() {
	if testBrokerStop != nil {
		log.Info().Msg("Shutting down test broker...")
		testBrokerStop()

		// Give broker time to shut down gracefully
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			if testBroker != nil {
				testBroker.BroadcastConnectionClose()
				testBroker.ActiveConns.Wait()
				testBroker.Shutdown()
			}
			close(done)
		}()

		select {
		case <-done:
			log.Info().Msg("Test broker shut down gracefully")
		case <-shutdownCtx.Done():
			log.Warn().Msg("Test broker shutdown timeout")
		}
	}
}
