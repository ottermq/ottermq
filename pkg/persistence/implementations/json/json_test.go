package json

import (
	"os"
	"reflect"
	"testing"

	"github.com/andrelcunha/ottermq/pkg/persistence"
)

func TestSafeVHostName(t *testing.T) {
	cases := map[string]string{
		"/":          "%2F",
		"vhost":      "vhost",
		"vhost/test": "vhost%2Ftest",
	}
	for input, expected := range cases {
		got := safeVHostName(input)
		if got != expected {
			t.Errorf("safeVHostName(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestSaveLoadExchange(t *testing.T) {
	// get temp dir for testing
	tempDir := t.TempDir()
	config := &persistence.Config{
		Type:    "json",
		DataDir: tempDir,
	}

	jsonPersistence, err := NewJsonPersistence(config)
	if err != nil {
		t.Fatalf("Failed to create JSON persistence: %v", err)
	}
	vhostName := "/"
	props := persistence.ExchangeProperties{
		Durable:    true,
		AutoDelete: false,
		Internal:   false,
		Arguments:  nil,
	}
	exchange := &JsonExchangeData{
		Name:       "test-exchange",
		Type:       "direct",
		Properties: props,
		Bindings:   []persistence.BindingData{{QueueName: "q1", RoutingKey: "rk", Arguments: nil}},
	}
	err = jsonPersistence.SaveExchangeMetadata(vhostName, exchange.Name, exchange.Type, exchange.Properties)
	if err != nil {
		t.Fatalf("SaveExchange failed: %v", err)
	}
	loadedType, loadedProps, err := jsonPersistence.LoadExchangeMetadata(vhostName, exchange.Name)
	if err != nil {
		t.Fatalf("LoadExchange failed: %v", err)
	}
	loaded := &JsonExchangeData{
		Name:       exchange.Name,
		Type:       loadedType,
		Properties: loadedProps,
		Bindings:   exchange.Bindings, // Bindings are not persisted in this implementation
	}
	// Compare saved and loaded exchange
	if !reflect.DeepEqual(exchange, loaded) {
		t.Errorf("Loaded exchange does not match saved.\nSaved: %+v\nLoaded: %+v", exchange, loaded)
	}
	// Cleanup
	safeName := safeVHostName(vhostName)
	os.RemoveAll("data/vhosts/" + safeName)
}

func TestSaveLoadQueue(t *testing.T) {
	// get temp dir for testing
	tempDir := t.TempDir()
	config := persistence.Config{
		Type:    "json",
		DataDir: tempDir,
	}
	jsonPersistence, err := NewJsonPersistence(&config)
	if err != nil {
		t.Fatalf("Failed to create JSON persistence: %v", err)
	}
	vhostName := "vhost/test"
	props := persistence.QueueProperties{
		Durable:    true,
		Exclusive:  false,
		AutoDelete: false,
		Arguments:  nil,
	}
	// msg := JsonMessageData{ID: "m1", Body: []byte("hello"), Properties: persistence.MessageProperties{DeliveryMode: 2}}
	queue := &JsonQueueData{
		Name:       "test-queue",
		Properties: props,
		// Messages:   []JsonMessageData{msg},
	}
	err = jsonPersistence.SaveQueueMetadata(vhostName, queue.Name, queue.Properties)
	if err != nil {
		t.Fatalf("SaveQueue failed: %v", err)
	}
	loadedProps, err := jsonPersistence.LoadQueueMetadata(vhostName, queue.Name)
	if err != nil {
		t.Fatalf("LoadQueue failed: %v", err)
	}

	// Compare only the properties since messages are not persisted in metadata
	if !reflect.DeepEqual(queue.Properties, loadedProps) {
		t.Errorf("Loaded queue properties do not match saved.\nSaved: %+v\nLoaded: %+v", queue.Properties, loadedProps)
	}
	// Cleanup
	safeName := safeVHostName(vhostName)
	os.RemoveAll("data/vhosts/" + safeName)
}

func TestDeleteExchange(t *testing.T) {
	// get temp dir for testing
	tempDir := t.TempDir()
	config := persistence.Config{
		Type:    "json",
		DataDir: tempDir,
	}
	jsonPersistence, err := NewJsonPersistence(&config)
	if err != nil {
		t.Fatalf("Failed to create JSON persistence: %v", err)
	}
	vhostName := "/"
	props := persistence.ExchangeProperties{
		Durable:    true,
		AutoDelete: false,
		Internal:   false,
		Arguments:  nil,
	}
	exchange := &JsonExchangeData{
		Name:       "delete-exchange",
		Type:       "direct",
		Properties: props,
		// Bindings:   []JsonBindingData{},
	}
	// Save exchange first
	err = jsonPersistence.SaveExchangeMetadata(vhostName, exchange.Name, exchange.Type, exchange.Properties)
	if err != nil {
		t.Fatalf("SaveExchange failed: %v", err)
	}
	// Delete exchange
	err = jsonPersistence.DeleteExchangeMetadata(vhostName, exchange.Name)
	if err != nil {
		t.Fatalf("DeleteExchange failed: %v", err)
	}
	// Try to load deleted exchange
	_, _, err = jsonPersistence.LoadExchangeMetadata(vhostName, exchange.Name)
	if err == nil {
		t.Errorf("Expected error when loading deleted exchange, got nil")
	}
	// Cleanup
	safeName := safeVHostName(vhostName)
	os.RemoveAll("data/vhosts/" + safeName)
}

func TestDeleteQueue(t *testing.T) {
	// get temp dir for testing
	tempDir := t.TempDir()
	config := persistence.Config{
		Type:    "json",
		DataDir: tempDir,
	}
	jsonPersistence, err := NewJsonPersistence(&config)
	if err != nil {
		t.Fatalf("Failed to create JSON persistence: %v", err)
	}
	vhostName := "vhost/test"
	props := persistence.QueueProperties{
		Durable:    true,
		Exclusive:  false,
		AutoDelete: false,
		Arguments:  nil,
	}
	// msg := JsonMessageData{ID: "m1", Body: []byte("hello"), Properties: persistence.MessageProperties{DeliveryMode: 2}}
	queue := &JsonQueueData{
		Name:       "delete-queue",
		Properties: props,
		// Messages:   []JsonMessageData{msg},
	}
	err = jsonPersistence.SaveQueueMetadata(vhostName, queue.Name, queue.Properties)
	if err != nil {
		t.Fatalf("SaveQueue failed: %v", err)
	}
	// Delete queue
	err = jsonPersistence.DeleteQueueMetadata(vhostName, queue.Name)
	if err != nil {
		t.Fatalf("DeleteQueue failed: %v", err)
	}
	// Try to load deleted queue
	_, err = jsonPersistence.LoadQueueMetadata(vhostName, queue.Name)
	if err == nil {
		t.Errorf("Expected error when loading deleted queue, got nil")
	}
	// Cleanup
	safeName := safeVHostName(vhostName)
	os.RemoveAll("data/vhosts/" + safeName)
}

func TestSaveLoadMessages(t *testing.T) {
	// get temp dir for testing
	tempDir := t.TempDir()
	config := persistence.Config{
		Type:    "json",
		DataDir: tempDir,
	}
	jsonPersistence, err := NewJsonPersistence(&config)
	if err != nil {
		t.Fatalf("Failed to create JSON persistence: %v", err)
	}

	vhostName := "/"
	queueName := "test-messages-queue"

	// Create queue first
	props := persistence.QueueProperties{
		Durable:    true,
		Exclusive:  false,
		AutoDelete: false,
		Arguments:  nil,
	}
	err = jsonPersistence.SaveQueueMetadata(vhostName, queueName, props)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	// Test saving messages
	messages := []persistence.Message{
		{
			ID:   "msg1",
			Body: []byte("Hello World 1"),
			Properties: persistence.MessageProperties{
				DeliveryMode: 2,
				ContentType:  "text/plain",
			},
		},
		{
			ID:   "msg2",
			Body: []byte("Hello World 2"),
			Properties: persistence.MessageProperties{
				DeliveryMode: 1,
				ContentType:  "application/json",
				Headers:      map[string]any{"test": "value"},
			},
		},
	}

	// Save messages
	for _, msg := range messages {
		err = jsonPersistence.SaveMessage(vhostName, queueName, msg.ID, msg.Body, msg.Properties)
		if err != nil {
			t.Fatalf("Failed to save message %s: %v", msg.ID, err)
		}
	}

	// Load messages and verify
	loadedMessages, err := jsonPersistence.LoadMessages(vhostName, queueName)
	if err != nil {
		t.Fatalf("Failed to load messages: %v", err)
	}

	if len(loadedMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(loadedMessages))
	}

	// Verify message content
	for i, expected := range messages {
		if i >= len(loadedMessages) {
			t.Errorf("Missing message at index %d", i)
			continue
		}

		actual := loadedMessages[i]
		if actual.ID != expected.ID {
			t.Errorf("Message %d: expected ID %s, got %s", i, expected.ID, actual.ID)
		}

		if string(actual.Body) != string(expected.Body) {
			t.Errorf("Message %d: expected body %s, got %s", i, string(expected.Body), string(actual.Body))
		}

		if actual.Properties.DeliveryMode != expected.Properties.DeliveryMode {
			t.Errorf("Message %d: expected delivery mode %d, got %d", i, expected.Properties.DeliveryMode, actual.Properties.DeliveryMode)
		}

		if actual.Properties.ContentType != expected.Properties.ContentType {
			t.Errorf("Message %d: expected content type %s, got %s", i, expected.Properties.ContentType, actual.Properties.ContentType)
		}
	}
}

func TestBindings_SaveLoad_Deduplicate_Delete(t *testing.T) {
	tempDir := t.TempDir()
	config := persistence.Config{Type: "json", DataDir: tempDir}
	jsonPersistence, err := NewJsonPersistence(&config)
	if err != nil {
		t.Fatalf("Failed to create JSON persistence: %v", err)
	}

	vhostName := "/"
	exchangeName := "ex-bindings"
	exProps := persistence.ExchangeProperties{Durable: true}

	// Save exchange metadata first
	if err := jsonPersistence.SaveExchangeMetadata(vhostName, exchangeName, "direct", exProps); err != nil {
		t.Fatalf("SaveExchangeMetadata failed: %v", err)
	}

	args := map[string]any{"x": 1}
	// Save single binding
	if err := jsonPersistence.SaveBindingState(vhostName, exchangeName, "q1", "rk1", args); err != nil {
		t.Fatalf("SaveBindingState failed: %v", err)
	}

	// Load and verify
	bindings, err := jsonPersistence.LoadExchangeBindings(vhostName, exchangeName)
	if err != nil {
		t.Fatalf("LoadExchangeBindings failed: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0].QueueName != "q1" || bindings[0].RoutingKey != "rk1" || !equalArgs(bindings[0].Arguments, args) {
		t.Errorf("unexpected binding loaded: %+v", bindings[0])
	}
	// Saving the same binding again should deduplicate (no duplicates added)
	if err := jsonPersistence.SaveBindingState(vhostName, exchangeName, "q1", "rk1", args); err != nil {
		t.Fatalf("SaveBindingState duplicate failed: %v", err)
	}
	bindings, err = jsonPersistence.LoadExchangeBindings(vhostName, exchangeName)
	if err != nil {
		t.Fatalf("LoadExchangeBindings failed: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding after duplicate save, got %d", len(bindings))
	}

	// Add a second, distinct binding
	if err := jsonPersistence.SaveBindingState(vhostName, exchangeName, "q2", "rk2", nil); err != nil {
		t.Fatalf("SaveBindingState (second) failed: %v", err)
	}
	bindings, err = jsonPersistence.LoadExchangeBindings(vhostName, exchangeName)
	if err != nil {
		t.Fatalf("LoadExchangeBindings failed: %v", err)
	}
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(bindings))
	}

	// Delete the second binding
	if err := jsonPersistence.DeleteBindingState(vhostName, exchangeName, "q2", "rk2", nil); err != nil {
		t.Fatalf("DeleteBindingState failed: %v", err)
	}
	bindings, err = jsonPersistence.LoadExchangeBindings(vhostName, exchangeName)
	if err != nil {
		t.Fatalf("LoadExchangeBindings failed: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding after delete, got %d", len(bindings))
	}
	if bindings[0].QueueName != "q1" || bindings[0].RoutingKey != "rk1" {
		t.Errorf("unexpected remaining binding: %+v", bindings[0])
	}
}

func TestBindings_SaveBindingRequiresExchange(t *testing.T) {
	tempDir := t.TempDir()
	config := persistence.Config{Type: "json", DataDir: tempDir}
	jsonPersistence, err := NewJsonPersistence(&config)
	if err != nil {
		t.Fatalf("Failed to create JSON persistence: %v", err)
	}

	vhostName := "/"
	// Intentionally do not save exchange metadata
	if err := jsonPersistence.SaveBindingState(vhostName, "non-existent", "q1", "rk1", nil); err == nil {
		t.Fatalf("expected error saving binding for missing exchange, got nil")
	}
}
