package prometheus

import "time"

type Config struct {
	Enabled        bool // Master swich
	Port           string
	UpdateInternal time.Duration
	Path           string // Endpoint path (default: /metrics)
}

func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		Port:           "9090",
		UpdateInternal: 5 * time.Second,
		Path:           "/metrics",
	}
}
