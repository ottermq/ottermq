package config

import (
	"os"
	"testing"
)

func TestLoadConfigWithDefaults(t *testing.T) {
	// Clear any environment variables that might interfere
	os.Clearenv()

	config := LoadConfig("test-version")

	// Check default values
	if config.BrokerPort != "5672" {
		t.Errorf("Expected Port to be '5672', got '%s'", config.BrokerPort)
	}
	if config.BrokerHost != "" {
		t.Errorf("Expected Host to be empty, got '%s'", config.BrokerHost)
	}
	if config.Username != "guest" {
		t.Errorf("Expected Username to be 'guest', got '%s'", config.Username)
	}
	if config.Password != "guest" {
		t.Errorf("Expected Password to be 'guest', got '%s'", config.Password)
	}
	if config.HeartbeatIntervalMax != 60 {
		t.Errorf("Expected HeartbeatIntervalMax to be 60, got %d", config.HeartbeatIntervalMax)
	}
	if config.ChannelMax != 2048 {
		t.Errorf("Expected ChannelMax to be 2048, got %d", config.ChannelMax)
	}
	if config.FrameMax != 131072 {
		t.Errorf("Expected FrameMax to be 131072, got %d", config.FrameMax)
	}
	if config.Ssl != false {
		t.Errorf("Expected Ssl to be false, got %t", config.Ssl)
	}
	if config.QueueBufferSize != 100000 {
		t.Errorf("Expected QueueBufferSize to be 100000, got %d", config.QueueBufferSize)
	}
	if config.WebPort != "3000" {
		t.Errorf("Expected WebServerPort to be '3000', got '%s'", config.WebPort)
	}
	if config.JwtSecret != "secret" {
		t.Errorf("Expected JwtSecret to be 'secret', got '%s'", config.JwtSecret)
	}
	if config.Version != "test-version" {
		t.Errorf("Expected Version to be 'test-version', got '%s'", config.Version)
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be 'info', got '%s'", config.LogLevel)
	}
	if config.EnableDLX != true {
		t.Errorf("Expected EnableDLX to be true, got %t", config.EnableDLX)
	}
	if config.EnableWebAPI != true {
		t.Errorf("Expected EnableWebAPI to be true, got %t", config.EnableWebAPI)
	}
	if config.EnableUI != true {
		t.Errorf("Expected EnableUI to be true, got %t", config.EnableUI)
	}
	if config.EnableSwagger != false {
		t.Errorf("Expected EnableSwagger to be false, got %t", config.EnableSwagger)
	}
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("OTTERMQ_BROKER_PORT", "15672")
	os.Setenv("OTTERMQ_BROKER_HOST", "localhost")

	os.Setenv("OTTERMQ_HEARTBEAT_INTERVAL", "120")
	os.Setenv("OTTERMQ_CHANNEL_MAX", "4096")
	os.Setenv("OTTERMQ_FRAME_MAX", "262144")
	os.Setenv("OTTERMQ_SSL", "true")
	os.Setenv("OTTERMQ_QUEUE_BUFFER_SIZE", "50000")

	os.Setenv("OTTERMQ_ENABLE_DLX", "false")
	os.Setenv("OTTERMQ_ENABLE_WEB_API", "false")
	os.Setenv("OTTERMQ_ENABLE_UI", "false")
	os.Setenv("OTTERMQ_ENABLE_SWAGGER", "true")

	os.Setenv("OTTERMQ_WEB_PORT", "8080")
	os.Setenv("OTTERMQ_USERNAME", "admin")
	os.Setenv("OTTERMQ_PASSWORD", "admin123")
	os.Setenv("OTTERMQ_JWT_SECRET", "my-secret-key")
	os.Setenv("LOG_LEVEL", "debug")

	defer func() {
		os.Clearenv()
	}()

	config := LoadConfig("env-version")

	// Check environment variable values
	if config.BrokerPort != "15672" {
		t.Errorf("Expected Port to be '15672', got '%s'", config.BrokerPort)
	}
	if config.BrokerHost != "localhost" {
		t.Errorf("Expected Host to be 'localhost', got '%s'", config.BrokerHost)
	}
	if config.Username != "admin" {
		t.Errorf("Expected Username to be 'admin', got '%s'", config.Username)
	}
	if config.Password != "admin123" {
		t.Errorf("Expected Password to be 'admin123', got '%s'", config.Password)
	}
	if config.HeartbeatIntervalMax != 120 {
		t.Errorf("Expected HeartbeatIntervalMax to be 120, got %d", config.HeartbeatIntervalMax)
	}
	if config.ChannelMax != 4096 {
		t.Errorf("Expected ChannelMax to be 4096, got %d", config.ChannelMax)
	}
	if config.FrameMax != 262144 {
		t.Errorf("Expected FrameMax to be 262144, got %d", config.FrameMax)
	}
	if config.Ssl != true {
		t.Errorf("Expected Ssl to be true, got %t", config.Ssl)
	}
	if config.QueueBufferSize != 50000 {
		t.Errorf("Expected QueueBufferSize to be 50000, got %d", config.QueueBufferSize)
	}
	if config.WebPort != "8080" {
		t.Errorf("Expected WebServerPort to be '8080', got '%s'", config.WebPort)
	}
	if config.JwtSecret != "my-secret-key" {
		t.Errorf("Expected JwtSecret to be 'my-secret-key', got '%s'", config.JwtSecret)
	}
	if config.Version != "env-version" {
		t.Errorf("Expected Version to be 'env-version', got '%s'", config.Version)
	}
	if config.LogLevel != "debug" {
		t.Errorf("Expected LogLevel to be 'debug', got '%s'", config.LogLevel)
	}
	if config.EnableDLX != false {
		t.Errorf("Expected EnableDLX to be false, got %t", config.EnableDLX)
	}
	if config.EnableWebAPI != false {
		t.Errorf("Expected EnableWebAPI to be false, got %t", config.EnableWebAPI)
	}
	if config.EnableUI != false {
		t.Errorf("Expected EnableUI to be false, got %t", config.EnableUI)
	}
	if config.EnableSwagger != true {
		t.Errorf("Expected EnableSwagger to be true, got %t", config.EnableSwagger)
	}
}

func TestLoadConfigWithInvalidEnvVars(t *testing.T) {
	// Set invalid environment variables
	os.Setenv("OTTERMQ_HEARTBEAT_INTERVAL", "invalid")
	os.Setenv("OTTERMQ_CHANNEL_MAX", "not-a-number")
	os.Setenv("OTTERMQ_FRAME_MAX", "xyz")
	os.Setenv("OTTERMQ_SSL", "maybe")
	os.Setenv("OTTERMQ_QUEUE_BUFFER_SIZE", "abc")

	defer func() {
		os.Clearenv()
	}()

	config := LoadConfig("invalid-version")

	// Should fall back to default values on invalid input
	if config.HeartbeatIntervalMax != 60 {
		t.Errorf("Expected HeartbeatIntervalMax to fall back to 60, got %d", config.HeartbeatIntervalMax)
	}
	if config.ChannelMax != 2048 {
		t.Errorf("Expected ChannelMax to fall back to 2048, got %d", config.ChannelMax)
	}
	if config.FrameMax != 131072 {
		t.Errorf("Expected FrameMax to fall back to 131072, got %d", config.FrameMax)
	}
	if config.Ssl != false {
		t.Errorf("Expected Ssl to fall back to false, got %t", config.Ssl)
	}
	if config.QueueBufferSize != 100000 {
		t.Errorf("Expected QueueBufferSize to fall back to 100000, got %d", config.QueueBufferSize)
	}
}
