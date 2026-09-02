package prometheus

import "time"

type Config struct {
	Enabled        bool // Master swich
	Port           string
	UpdateInterval time.Duration
	Path           string // Endpoint path (default: /metrics)
}

func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		Port:           "9090",
		UpdateInterval: 5 * time.Second,
		Path:           "/metrics",
	}
}
