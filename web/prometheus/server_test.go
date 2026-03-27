package prometheus

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ottermq/ottermq/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusServer(t *testing.T) {
	// Setup
	ctx := t.Context()
	collector := metrics.NewCollector(metrics.DefaultConfig(), ctx)
	config := &Config{
		Enabled:        true,
		Port:           "19090", // Use different port for testing
		UpdateInterval: 100 * time.Millisecond,
		Path:           "/metrics",
	}

	exporter := NewExporter(collector, config)
	server := NewServer(config, exporter)

	// Start server in goroutine
	go func() {
		_ = server.Start()
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)
	defer server.Shutdown()

	// Test metrics endpoint
	resp, err := http.Get("http://localhost:19090/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify Prometheus format
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "ottermq_messages_published_total")
	assert.Contains(t, bodyStr, "ottermq_connection_count")
}

// func TestExporterRecording(t *testing.T) {
// 	ctx := t.Context()
// 	collector := metrics.NewCollector(metrics.DefaultConfig(), ctx)
// 	config := DefaultConfig()
// 	exporter := NewExporter(collector, config)

// 	// Record some events
// 	exporter.RecordPublish()
// 	exporter.RecordPublish()
// 	exporter.RecordDelivery(100)
// 	exporter.RecordAck()

// 	// Metrics should be updated (checked via Prometheus registry)
// 	// This is a basic test; in production, you'd query the registry
// }
