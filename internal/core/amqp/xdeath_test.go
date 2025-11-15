package amqp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncodeDecodeXDeathHeader tests encoding and decoding of x-death header structure
func TestEncodeDecodeXDeathHeader(t *testing.T) {
	// Create an x-death entry similar to what we use

	// Create x-death array
	xDeathArray := []map[string]any{{
		"queue":               "test-queue",
		"reason":              "rejected",
		"count":               uint32(1),
		"time":                "2025-11-12T12:00:00Z",
		"exchange":            "test-exchange",
		"routing-keys":        []string{"key1", "key2"},
		"original-expiration": "",
	}}

	// Create headers with x-death
	headers := map[string]any{
		"x-death":                xDeathArray,
		"x-first-death-queue":    "test-queue",
		"x-first-death-reason":   "rejected",
		"x-first-death-exchange": "test-exchange",
		"x-last-death-queue":     "test-queue",
		"x-last-death-reason":    "rejected",
		"x-last-death-exchange":  "test-exchange",
	}

	// Encode
	encoded := EncodeTable(headers)
	require.NotEmpty(t, encoded, "Encoded table should not be empty")

	t.Logf("Encoded length: %d bytes", len(encoded))

	// Decode
	decoded, err := DecodeTable(encoded)
	require.NoError(t, err, "Should decode without error")

	// Verify x-death
	xDeath, ok := decoded["x-death"].([]any)
	require.True(t, ok, "x-death should be decoded as []any")
	require.Len(t, xDeath, 1, "x-death should have 1 entry")

	// Verify first entry
	entry, ok := xDeath[0].(map[string]any)
	require.True(t, ok, "x-death entry should be a map")

	assert.Equal(t, "test-queue", entry["queue"])
	assert.Equal(t, "rejected", entry["reason"])
	assert.Equal(t, uint32(1), entry["count"]) // AMQP client decodes as uint32
	assert.Equal(t, "test-exchange", entry["exchange"])

	// Verify routing-keys
	routingKeys, ok := entry["routing-keys"].([]any)
	require.True(t, ok, "routing-keys should be an array")
	require.Len(t, routingKeys, 2)
	assert.Equal(t, "key1", routingKeys[0])
	assert.Equal(t, "key2", routingKeys[1])
}
