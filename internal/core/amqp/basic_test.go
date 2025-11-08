package amqp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseBasicQosFrame(t *testing.T) {
	tests := []struct {
		name                  string
		payload               []byte
		expectedPrefetchSize  uint32
		expectedPrefetchCount uint16
		expectedGlobal        bool
		shouldError           bool
		errorMsg              string
	}{
		{
			name: "Valid basic qos with 0 prefetch, 0 consumer and global false",
			payload: buildBasicQosPayload(t, BasicQosContent{
				PrefetchSize:  0,
				PrefetchCount: 0,
				Global:        false,
			}),
			expectedPrefetchSize:  0,
			expectedPrefetchCount: 0,
			expectedGlobal:        false,
			shouldError:           false,
		},
		{
			name:        "Invalid basic qos with short payload",
			payload:     []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
			shouldError: true,
			errorMsg:    "payload too short",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function under test
			result, err := parseBasicQosFrame(tt.payload)

			// Validate the results
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s' but got '%s'", tt.errorMsg, err.Error())
					return
				}
				return
			}

			if result == nil {
				t.Errorf("Expected no error but got one")
				return
			}
			content, ok := result.Content.(*BasicQosContent)
			if !ok {
				t.Errorf("Expected content of type *BasicQosContent but got %T", result.Content)
				return
			}
			if content.PrefetchSize != tt.expectedPrefetchSize {
				t.Errorf("Expected prefetch_size %d but got %d", tt.expectedPrefetchSize, content.PrefetchSize)
			}
			if content.PrefetchCount != tt.expectedPrefetchCount {
				t.Errorf("Expected prefetch_count %d but got %d", tt.expectedPrefetchCount, content.PrefetchCount)
			}
			if content.Global != tt.expectedGlobal {
				t.Errorf("Expected global %v but got %v", tt.expectedGlobal, content.Global)
			}

		})
	}

}

// Helper function to build BASIC_QOS payload
func buildBasicQosPayload(t *testing.T, params BasicQosContent) []byte {
	var buf bytes.Buffer

	// prefetch_size (long)
	if err := binary.Write(&buf, binary.BigEndian, params.PrefetchSize); err != nil {
		t.Fatalf("Failed to write prefetch_size: %v", err)
	}

	// prefetch_count (short)
	if err := binary.Write(&buf, binary.BigEndian, params.PrefetchCount); err != nil {
		t.Fatalf("Failed to write prefetch_count: %v", err)
	}

	// global (bit)
	flags := map[string]bool{
		"global": params.Global,
	}
	var globalBit byte = EncodeFlags(flags, []string{"global"}, true)
	if err := buf.WriteByte(globalBit); err != nil {
		t.Fatalf("Failed to write global: %v", err)
	}

	return buf.Bytes()
}

func TestParseBasicConsumeFrame(t *testing.T) {
	tests := []struct {
		name              string
		payload           []byte
		expectedQueue     string
		expectedTag       string
		expectedNoLocal   bool
		expectedNoAck     bool
		expectedExclusive bool
		expectedNoWait    bool
		expectedArgs      map[string]any
		shouldError       bool
		errorMsg          string
	}{
		{
			name: "Valid basic consume with empty queue and tag",
			payload: buildBasicConsumePayload(t, BasicConsumeParams{
				Queue:       "",
				ConsumerTag: "",
				NoLocal:     false,
				NoAck:       false,
				Exclusive:   false,
				NoWait:      false,
				Arguments:   nil,
			}),
			expectedQueue:     "",
			expectedTag:       "",
			expectedNoLocal:   false,
			expectedNoAck:     false,
			expectedExclusive: false,
			expectedNoWait:    false,
			expectedArgs:      nil,
			shouldError:       false,
		},
		{
			name: "Valid basic consume with queue and consumer tag",
			payload: buildBasicConsumePayload(t, BasicConsumeParams{
				Queue:       "test-queue",
				ConsumerTag: "consumer-1",
				NoLocal:     false,
				NoAck:       true,
				Exclusive:   false,
				NoWait:      false,
				Arguments:   nil,
			}),
			expectedQueue:     "test-queue",
			expectedTag:       "consumer-1",
			expectedNoLocal:   false,
			expectedNoAck:     true,
			expectedExclusive: false,
			expectedNoWait:    false,
			expectedArgs:      nil,
			shouldError:       false,
		},
		{
			name: "Valid basic consume with all flags set",
			payload: buildBasicConsumePayload(t, BasicConsumeParams{
				Queue:       "exclusive-queue",
				ConsumerTag: "exclusive-consumer",
				NoLocal:     true,
				NoAck:       true,
				Exclusive:   true,
				NoWait:      true,
				Arguments:   nil,
			}),
			expectedQueue:     "exclusive-queue",
			expectedTag:       "exclusive-consumer",
			expectedNoLocal:   true,
			expectedNoAck:     true,
			expectedExclusive: true,
			expectedNoWait:    true,
			expectedArgs:      nil,
			shouldError:       false,
		},
		{
			name: "Valid basic consume with arguments",
			payload: buildBasicConsumePayload(t, BasicConsumeParams{
				Queue:       "queue-with-args",
				ConsumerTag: "consumer-with-args",
				NoLocal:     false,
				NoAck:       false,
				Exclusive:   false,
				NoWait:      false,
				Arguments: map[string]any{
					"x-priority":    int32(10),
					"x-message-ttl": int32(60000),
				},
			}),
			expectedQueue:     "queue-with-args",
			expectedTag:       "consumer-with-args",
			expectedNoLocal:   false,
			expectedNoAck:     false,
			expectedExclusive: false,
			expectedNoWait:    false,
			expectedArgs: map[string]any{
				"x-priority":    int32(10),
				"x-message-ttl": int32(60000),
			},
			shouldError: false,
		},
		{
			name: "Valid basic consume with complex arguments",
			payload: buildBasicConsumePayload(t, BasicConsumeParams{
				Queue:       "complex-queue",
				ConsumerTag: "complex-consumer",
				NoLocal:     false,
				NoAck:       false,
				Exclusive:   false,
				NoWait:      false,
				Arguments: map[string]any{
					"x-prefetch-count":        int32(100),
					"x-consumer-timeout":      int32(30000),
					"x-cancel-on-ha-failover": true,
				},
			}),
			expectedQueue:     "complex-queue",
			expectedTag:       "complex-consumer",
			expectedNoLocal:   false,
			expectedNoAck:     false,
			expectedExclusive: false,
			expectedNoWait:    false,
			expectedArgs: map[string]any{
				"x-prefetch-count":        int32(100),
				"x-consumer-timeout":      int32(30000),
				"x-cancel-on-ha-failover": true,
			},
			shouldError: false,
		},
		{
			name:        "Payload too short",
			payload:     []byte{0x00, 0x00, 0x01, 'q'}, // Only 4 bytes
			shouldError: true,
			errorMsg:    "payload too short",
		},
		{
			name:        "Empty payload",
			payload:     []byte{},
			shouldError: true,
			errorMsg:    "payload too short",
		},
		{
			name: "Minimum valid payload without arguments",
			payload: buildBasicConsumePayload(t, BasicConsumeParams{
				Queue:       "",
				ConsumerTag: "",
				NoLocal:     false,
				NoAck:       false,
				Exclusive:   false,
				NoWait:      false,
				Arguments:   nil,
			}),
			expectedQueue:     "",
			expectedTag:       "",
			expectedNoLocal:   false,
			expectedNoAck:     false,
			expectedExclusive: false,
			expectedNoWait:    false,
			expectedArgs:      nil,
			shouldError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseBasicConsumeFrame(tt.payload)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result but got nil")
				return
			}

			content, ok := result.Content.(*BasicConsumeContent)
			if !ok {
				t.Errorf("Expected BasicConsumeContent, got %T", result.Content)
				return
			}

			// Validate all fields
			if content.Queue != tt.expectedQueue {
				t.Errorf("Expected queue '%s', got '%s'", tt.expectedQueue, content.Queue)
			}
			if content.ConsumerTag != tt.expectedTag {
				t.Errorf("Expected consumer tag '%s', got '%s'", tt.expectedTag, content.ConsumerTag)
			}
			if content.NoLocal != tt.expectedNoLocal {
				t.Errorf("Expected NoLocal %t, got %t", tt.expectedNoLocal, content.NoLocal)
			}
			if content.NoAck != tt.expectedNoAck {
				t.Errorf("Expected NoAck %t, got %t", tt.expectedNoAck, content.NoAck)
			}
			if content.Exclusive != tt.expectedExclusive {
				t.Errorf("Expected Exclusive %t, got %t", tt.expectedExclusive, content.Exclusive)
			}
			if content.NoWait != tt.expectedNoWait {
				t.Errorf("Expected NoWait %t, got %t", tt.expectedNoWait, content.NoWait)
			}

			// Validate arguments
			if tt.expectedArgs == nil {
				if content.Arguments != nil {
					t.Errorf("Expected nil arguments, got %v", content.Arguments)
				}
			} else {
				if content.Arguments == nil {
					t.Error("Expected arguments but got nil")
					return
				}
				for key, expectedValue := range tt.expectedArgs {
					actualValue, exists := content.Arguments[key]
					if !exists {
						t.Errorf("Expected argument '%s' not found", key)
						continue
					}
					if actualValue != expectedValue {
						t.Errorf("Expected argument '%s' value %v, got %v", key, expectedValue, actualValue)
					}
				}
				// Check for unexpected arguments
				for key := range content.Arguments {
					if _, exists := tt.expectedArgs[key]; !exists {
						t.Errorf("Unexpected argument '%s' found", key)
					}
				}
			}
		})
	}
}

// Helper struct for building test payloads
type BasicConsumeParams struct {
	Queue       string
	ConsumerTag string
	NoLocal     bool
	NoAck       bool
	Exclusive   bool
	NoWait      bool
	Arguments   map[string]any
}

// Helper function to build BASIC_CONSUME payload
func buildBasicConsumePayload(t *testing.T, params BasicConsumeParams) []byte {
	var buf bytes.Buffer

	// reserved1 (short int)
	if err := binary.Write(&buf, binary.BigEndian, uint16(0)); err != nil {
		t.Fatalf("Failed to write reserved1: %v", err)
	}

	// queue (shortstr)
	if err := buf.WriteByte(byte(len(params.Queue))); err != nil {
		t.Fatalf("Failed to write queue length: %v", err)
	}
	if _, err := buf.WriteString(params.Queue); err != nil {
		t.Fatalf("Failed to write queue: %v", err)
	}

	// consumer-tag (shortstr)
	if err := buf.WriteByte(byte(len(params.ConsumerTag))); err != nil {
		t.Fatalf("Failed to write consumer tag length: %v", err)
	}
	if _, err := buf.WriteString(params.ConsumerTag); err != nil {
		t.Fatalf("Failed to write consumer tag: %v", err)
	}

	// flags (octet)
	var flags uint8
	if params.NoLocal {
		flags |= 0x01 // bit 0
	}
	if params.NoAck {
		flags |= 0x02 // bit 1
	}
	if params.Exclusive {
		flags |= 0x04 // bit 2
	}
	if params.NoWait {
		flags |= 0x08 // bit 3
	}
	if err := buf.WriteByte(flags); err != nil {
		t.Fatalf("Failed to write flags: %v", err)
	}

	// arguments (table encoded as longstr)
	if params.Arguments != nil {
		tableData := EncodeTable(params.Arguments)

		// Write table length (4 bytes)
		if err := binary.Write(&buf, binary.BigEndian, uint32(len(tableData))); err != nil {
			t.Fatalf("Failed to write arguments length: %v", err)
		}

		// Write table data
		if _, err := buf.Write(tableData); err != nil {
			t.Fatalf("Failed to write arguments data: %v", err)
		}
	}
	// If Arguments is nil, don't write anything for the table

	return buf.Bytes()
}

func TestParseBasicConsumeFrame_EdgeCases(t *testing.T) {
	t.Run("Maximum length queue name", func(t *testing.T) {
		longQueue := string(make([]byte, 255)) // Maximum shortstr length
		for i := range longQueue {
			longQueue = longQueue[:i] + "q" + longQueue[i+1:]
		}

		payload := buildBasicConsumePayload(t, BasicConsumeParams{
			Queue:       longQueue,
			ConsumerTag: "consumer",
			Arguments:   nil,
		})

		result, err := parseBasicConsumeFrame(payload)
		if err != nil {
			t.Errorf("Unexpected error with max length queue: %v", err)
			return
		}

		content := result.Content.(*BasicConsumeContent)
		if content.Queue != longQueue {
			t.Error("Queue name was not preserved correctly")
		}
	})

	t.Run("Maximum length consumer tag", func(t *testing.T) {
		longTag := string(make([]byte, 255)) // Maximum shortstr length
		for i := range longTag {
			longTag = longTag[:i] + "c" + longTag[i+1:]
		}

		payload := buildBasicConsumePayload(t, BasicConsumeParams{
			Queue:       "test",
			ConsumerTag: longTag,
			Arguments:   nil,
		})

		result, err := parseBasicConsumeFrame(payload)
		if err != nil {
			t.Errorf("Unexpected error with max length consumer tag: %v", err)
			return
		}

		content := result.Content.(*BasicConsumeContent)
		if content.ConsumerTag != longTag {
			t.Error("Consumer tag was not preserved correctly")
		}
	})

	t.Run("All flags combinations", func(t *testing.T) {
		// Test all 16 possible combinations of the 4 flags
		for i := 0; i < 16; i++ {
			noLocal := (i & 8) != 0
			noAck := (i & 4) != 0
			exclusive := (i & 2) != 0
			noWait := (i & 1) != 0

			payload := buildBasicConsumePayload(t, BasicConsumeParams{
				Queue:       "test",
				ConsumerTag: "consumer",
				NoLocal:     noLocal,
				NoAck:       noAck,
				Exclusive:   exclusive,
				NoWait:      noWait,
				Arguments:   nil,
			})

			result, err := parseBasicConsumeFrame(payload)
			if err != nil {
				t.Errorf("Error with flags combination %d: %v", i, err)
				continue
			}

			content := result.Content.(*BasicConsumeContent)
			if content.NoLocal != noLocal ||
				content.NoAck != noAck ||
				content.Exclusive != exclusive ||
				content.NoWait != noWait {
				t.Errorf("Flags not parsed correctly for combination %d", i)
			}
		}
	})
}

func TestParseBasicConsumeFrame_ArgumentsEdgeCases(t *testing.T) {
	t.Run("Empty arguments table", func(t *testing.T) {
		payload := buildBasicConsumePayload(t, BasicConsumeParams{
			Queue:       "test",
			ConsumerTag: "consumer",
			Arguments:   map[string]any{}, // Empty but not nil
		})

		result, err := parseBasicConsumeFrame(payload)
		if err != nil {
			t.Errorf("Unexpected error with empty arguments: %v", err)
			return
		}

		content := result.Content.(*BasicConsumeContent)
		if content.Arguments == nil {
			t.Error("Expected empty map, got nil")
		}
	})

	t.Run("Arguments with various types", func(t *testing.T) {
		args := map[string]any{
			"string-arg": "test-value",
			"int-arg":    int32(42),
			"bool-arg":   true,
		}

		payload := buildBasicConsumePayload(t, BasicConsumeParams{
			Queue:       "test",
			ConsumerTag: "consumer",
			Arguments:   args,
		})

		result, err := parseBasicConsumeFrame(payload)
		if err != nil {
			t.Errorf("Unexpected error with mixed arguments: %v", err)
			return
		}

		content := result.Content.(*BasicConsumeContent)
		for key, expectedValue := range args {
			if actualValue, exists := content.Arguments[key]; !exists {
				t.Errorf("Argument '%s' not found", key)
			} else if actualValue != expectedValue {
				t.Errorf("Argument '%s': expected %v, got %v", key, expectedValue, actualValue)
			}
		}
	})
}

func TestCreateBasicDeliverFrame(t *testing.T) {
	tests := []struct {
		name        string
		channel     uint16
		consumerTag string
		exchange    string
		routingKey  string
		deliveryTag uint64
		redelivered bool
	}{
		{
			name:        "Standard deliver frame",
			channel:     1,
			consumerTag: "test-consumer",
			exchange:    "test-exchange",
			routingKey:  "test.routing.key",
			deliveryTag: 123,
			redelivered: false,
		},
		{
			name:        "Redelivered message",
			channel:     2,
			consumerTag: "consumer-2",
			exchange:    "amq.direct",
			routingKey:  "redelivered.key",
			deliveryTag: 456,
			redelivered: true,
		},
		{
			name:        "Empty exchange and routing key",
			channel:     0,
			consumerTag: "empty-consumer",
			exchange:    "",
			routingKey:  "",
			deliveryTag: 0,
			redelivered: false,
		},
		{
			name:        "High delivery tag",
			channel:     65535,
			consumerTag: "high-tag-consumer",
			exchange:    "high.exchange",
			routingKey:  "high.routing",
			deliveryTag: 18446744073709551615, // max uint64
			redelivered: true,
		},
		{
			name:        "Long consumer tag",
			channel:     10,
			consumerTag: "very-long-consumer-tag-with-many-characters",
			exchange:    "long.exchange.name.with.dots",
			routingKey:  "long.routing.key.with.many.segments",
			deliveryTag: 999999,
			redelivered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function under test
			frame := createBasicDeliverFrame(tt.channel, tt.consumerTag, tt.exchange, tt.routingKey, tt.deliveryTag, tt.redelivered)

			// Verify frame is not empty
			if len(frame) == 0 {
				t.Error("Expected non-empty frame")
				return
			}

			// Basic frame structure validation - should start with frame type and channel
			if len(frame) < 8 { // Minimum AMQP frame header size
				t.Errorf("Frame too short: %d bytes", len(frame))
				return
			}

			// Verify frame type (METHOD frame = 1)
			if frame[0] != 1 {
				t.Errorf("Expected frame type 1 (METHOD), got %d", frame[0])
			}

			// Verify channel (bytes 1-2, big-endian)
			frameChannel := uint16(frame[1])<<8 | uint16(frame[2])
			if frameChannel != tt.channel {
				t.Errorf("Expected channel %d, got %d", tt.channel, frameChannel)
			}

			// Verify the frame contains expected class and method IDs
			// This would require parsing the full frame, but we can at least
			// verify the frame was created without panicking and has reasonable size
			expectedMinSize := 7 + // frame header (1 byte type + 2 bytes channel + 4 bytes payload size)
				2 + // class ID
				2 + // method ID
				1 + len(tt.consumerTag) + // consumer tag (shortstr)
				8 + // delivery tag (longlong)
				1 + // redelivered (bit, packed in octet)
				1 + len(tt.exchange) + // exchange (shortstr)
				1 + len(tt.routingKey) + // routing key (shortstr)
				1 // frame end

			if len(frame) < expectedMinSize {
				t.Errorf("Frame smaller than expected minimum size. Got %d, expected at least %d", len(frame), expectedMinSize)
			}

			// Verify frame ends with frame-end byte (0xCE)
			if frame[len(frame)-1] != 0xCE {
				t.Errorf("Expected frame to end with 0xCE, got 0x%02X", frame[len(frame)-1])
			}
		})
	}
}

func TestCreateBasicDeliverFrame_ContentStructure(t *testing.T) {
	// Test that the function creates the correct internal structure
	channel := uint16(5)
	consumerTag := "test-consumer"
	exchange := "test-exchange"
	routingKey := "test.key"
	deliveryTag := uint64(789)
	redelivered := true

	frame := createBasicDeliverFrame(channel, consumerTag, exchange, routingKey, deliveryTag, redelivered)

	// Verify frame is created
	if frame == nil {
		t.Fatal("Frame should not be nil")
	}

	if len(frame) == 0 {
		t.Fatal("Frame should not be empty")
	}

	// The function should create a ResponseMethodMessage internally with:
	// - Channel: 5
	// - ClassID: BASIC (60)
	// - MethodID: BASIC_DELIVER
	// - Content: ContentList with 5 KeyValue pairs

	// We can't directly inspect the internal structure without exposing it,
	// but we can verify the frame has the expected characteristics
	t.Logf("Generated frame length: %d bytes", len(frame))
	t.Logf("Frame starts with: %v", frame[:min(10, len(frame))])
	t.Logf("Frame ends with: %v", frame[max(0, len(frame)-5):])
}

func TestCreateBasicDeliverFrame_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		channel     uint16
		consumerTag string
		exchange    string
		routingKey  string
		deliveryTag uint64
		redelivered bool
		description string
	}{
		{
			name:        "Zero values",
			channel:     0,
			consumerTag: "",
			exchange:    "",
			routingKey:  "",
			deliveryTag: 0,
			redelivered: false,
			description: "All minimum/zero values should work",
		},
		{
			name:        "Single character strings",
			channel:     1,
			consumerTag: "c",
			exchange:    "e",
			routingKey:  "r",
			deliveryTag: 1,
			redelivered: true,
			description: "Single character strings should work",
		},
		{
			name:        "Max channel value",
			channel:     65535,
			consumerTag: "max-channel-consumer",
			exchange:    "max.exchange",
			routingKey:  "max.key",
			deliveryTag: 12345,
			redelivered: false,
			description: "Maximum channel value should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Function panicked: %v", r)
				}
			}()

			frame := createBasicDeliverFrame(tt.channel, tt.consumerTag, tt.exchange, tt.routingKey, tt.deliveryTag, tt.redelivered)

			if len(frame) == 0 {
				t.Errorf("Expected non-empty frame for case: %s", tt.description)
			}

			// Verify basic frame structure
			if len(frame) < 8 {
				t.Errorf("Frame too short for case: %s", tt.description)
			}

			// Verify frame type
			if frame[0] != 1 {
				t.Errorf("Expected METHOD frame type for case: %s", tt.description)
			}

			// Verify frame end marker
			if frame[len(frame)-1] != 0xCE {
				t.Errorf("Expected frame end marker for case: %s", tt.description)
			}
		})
	}
}

func TestParseBasicCancelFrame_ValidPayload(t *testing.T) {
	// Create a payload with consumer tag "test-consumer" and nowait=false
	var payload []byte

	// Consumer tag: "test-consumer" (12 bytes + 1 length byte)
	consumerTag := "test-consumer"
	payload = append(payload, byte(len(consumerTag)))
	payload = append(payload, []byte(consumerTag)...)

	// Flags: nowait=false (bit 0 = 0)
	payload = append(payload, 0x00)

	result, err := parseBasicCancelFrame(payload)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	content, ok := result.Content.(*BasicCancelContent)
	if !ok {
		t.Fatalf("Expected BasicCancelContent, got %T", result.Content)
	}

	if content.ConsumerTag != "test-consumer" {
		t.Errorf("Expected consumer tag 'test-consumer', got '%s'", content.ConsumerTag)
	}

	if content.NoWait {
		t.Error("Expected Nowait to be false")
	}
}

func TestParseBasicCancelFrame_WithNowaitFlag(t *testing.T) {
	// Create a payload with consumer tag "consumer1" and nowait=true
	var payload []byte

	// Consumer tag: "consumer1"
	consumerTag := "consumer1"
	payload = append(payload, byte(len(consumerTag)))
	payload = append(payload, []byte(consumerTag)...)

	// Flags: nowait=true (bit 0 = 1)
	payload = append(payload, 0x01)

	result, err := parseBasicCancelFrame(payload)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	content, ok := result.Content.(*BasicCancelContent)
	if !ok {
		t.Fatalf("Expected BasicCancelContent, got %T", result.Content)
	}

	if content.ConsumerTag != "consumer1" {
		t.Errorf("Expected consumer tag 'consumer1', got '%s'", content.ConsumerTag)
	}

	if !content.NoWait {
		t.Error("Expected Nowait to be true")
	}
}

func TestParseBasicCancelFrame_PayloadTooShort(t *testing.T) {
	// Payload with only 1 byte (minimum is 1)
	payload := []byte{0x00} // length=0

	result, err := parseBasicCancelFrame(payload)

	if err == nil {
		t.Error("Expected error for payload too short")
	}

	expectedError := "payload too short"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	if result != nil {
		t.Error("Expected nil result")
	}
}

func TestParseBasicCancelFrame_LongConsumerTag(t *testing.T) {
	// Create payload with long consumer tag (255 chars - maximum for short string)
	consumerTag := string(make([]byte, 255))
	for i := range consumerTag {
		consumerTag = consumerTag[:i] + "a" + consumerTag[i+1:]
	}

	var payload []byte
	payload = append(payload, byte(len(consumerTag)))
	payload = append(payload, []byte(consumerTag)...)
	payload = append(payload, 0x01) // nowait=true

	result, err := parseBasicCancelFrame(payload)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	content, ok := result.Content.(*BasicCancelContent)
	if !ok {
		t.Fatalf("Expected BasicCancelContent, got %T", result.Content)
	}

	if content.ConsumerTag != consumerTag {
		t.Errorf("Expected consumer tag length %d, got %d", len(consumerTag), len(content.ConsumerTag))
	}

	if !content.NoWait {
		t.Error("Expected Nowait to be true")
	}
}

func TestParseBasicCancelFrame_MissingFlagsByte(t *testing.T) {
	// Create payload missing the flags byte
	var payload []byte

	// Consumer tag: "test"
	consumerTag := "test"
	payload = append(payload, byte(len(consumerTag)))
	payload = append(payload, []byte(consumerTag)...)
	// Missing flags byte

	result, err := parseBasicCancelFrame(payload)

	if err == nil {
		t.Error("Expected error for missing flags byte")
	}

	if !bytes.Contains([]byte(err.Error()), []byte("failed to read octet")) {
		t.Errorf("Expected error about reading octet, got: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result")
	}
}

func TestParseBasicCancelFrame_SingleCharacterTag(t *testing.T) {
	// Test with minimum valid consumer tag (1 character)
	var payload []byte

	// Consumer tag: "x"
	consumerTag := "x"
	payload = append(payload, byte(len(consumerTag)))
	payload = append(payload, []byte(consumerTag)...)
	payload = append(payload, 0x00) // nowait=false

	result, err := parseBasicCancelFrame(payload)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	content, ok := result.Content.(*BasicCancelContent)
	if !ok {
		t.Fatalf("Expected BasicCancelContent, got %T", result.Content)
	}

	if content.ConsumerTag != "x" {
		t.Errorf("Expected consumer tag 'x', got '%s'", content.ConsumerTag)
	}

	if content.NoWait {
		t.Error("Expected Nowait to be false")
	}
}
