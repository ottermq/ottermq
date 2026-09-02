# Logger Package

This package provides centralized structured logging for OtterMQ using [Zerolog](https://github.com/rs/zerolog).

## Features

- **Zero-allocation logging**: High-performance structured logging with minimal overhead
- **Configurable log levels**: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC
- **Environment variable configuration**: Set log level via `LOG_LEVEL` env var
- **Human-readable console output**: Pretty-printed logs for development
- **Metrics/Observability hooks**: Built-in support for future Prometheus integration

## Usage

### Basic Usage

Initialize the logger at application startup:

```go
import "github.com/ottermq/ottermq/pkg/logger"

func main() {
    // Initialize with log level from config
    logger.Init("info")
    
    // Use the global logger
    log.Info().Msg("Application started")
}
```

### Structured Logging

```go
import "github.com/rs/zerolog/log"

// Before (old style)
log.Println("[DEBUG] New client waiting for connection: ", conn.RemoteAddr())

// After (structured)
log.Debug().Str("client", conn.RemoteAddr().String()).Msg("New client waiting for connection")
```

### Log Levels

```go
log.Trace().Msg("Detailed trace message")
log.Debug().Str("key", "value").Msg("Debug message")
log.Info().Int("count", 42).Msg("Info message")
log.Warn().Err(err).Msg("Warning message")
log.Error().Err(err).Msg("Error message")
log.Fatal().Msg("Fatal error - exits application")
```

### Configuration

Set the log level via environment variable:

```bash
LOG_LEVEL=debug ./ottermq
```

Available levels: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`

## Metrics/Observability Integration

The logger package includes support for metrics hooks, paving the way for future Prometheus integration:

```go
import (
    "github.com/ottermq/ottermq/pkg/logger"
    "github.com/rs/zerolog"
)

// Example hook for metrics (to be implemented)
metricsHook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, msg string) {
    // Increment Prometheus counters based on log level
    // logCounter.WithLabelValues(level.String()).Inc()
})

// Set the hook before initializing
logger.SetMetricsHook(metricsHook)
logger.Init("info")
```

This architecture allows for:
- **Log-based metrics**: Count errors, warnings, and other log events
- **Performance monitoring**: Track log rates and patterns
- **Alerting**: Trigger alerts based on error log frequency
- **Future Prometheus integration**: Ready for metrics collection

## Testing

The package includes comprehensive tests:

```bash
go test ./pkg/logger -v
```

## Design Goals

1. **Performance**: Zero-allocation design for minimal overhead
2. **Minimal footprint**: Small dependency and runtime footprint
3. **Structured output**: Machine-readable JSON logs for parsing
4. **Developer experience**: Human-readable console output
5. **Extensibility**: Hook system for metrics and observability
