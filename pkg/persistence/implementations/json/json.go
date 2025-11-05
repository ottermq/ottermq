package json

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/andrelcunha/ottermq/pkg/persistence"
)

type JsonPersistence struct {
	dataDir string
}

func NewJsonPersistence(config *persistence.Config) (*JsonPersistence, error) {
	jp := &JsonPersistence{
		dataDir: config.DataDir,
	}
	return jp, jp.Initialize()
}

// equalArgs compares two argument maps by their JSON encoding to avoid type
// discrepancies (e.g., int vs float64) after JSON round-trips.
func equalArgs(a, b map[string]any) bool {
	// Quick path
	if a == nil && b == nil {
		return true
	}
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}

func (jp *JsonPersistence) Initialize() error {
	// Create the base data directory if it doesn't exist
	if err := os.MkdirAll(jp.dataDir, 0755); err != nil {
		return err
	}
	return nil
}

func (jp *JsonPersistence) Close() error {
	// JSON implementation doesn't need to clean up
	return nil
}

// safeVHostName encodes vhost names for safe filesystem usage
func safeVHostName(name string) string {
	return url.PathEscape(name)
}

// SaveExchangeMetadata persists exchange metadata to a JSON file
func (jp *JsonPersistence) SaveExchangeMetadata(vhost, name, exchangeType string, props persistence.ExchangeProperties) error {
	safeName := safeVHostName(vhost)
	dir := filepath.Join(jp.dataDir, "vhosts", safeName, "exchanges")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	exchangeData := JsonExchangeData{
		Name:       name,
		Type:       exchangeType,
		Properties: props,
	}
	file := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(exchangeData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

// LoadExchangeMetadata loads exchange metadata from a JSON file (type and properties)
func (jp *JsonPersistence) LoadExchangeMetadata(vhost, name string) (exchangeType string, props persistence.ExchangeProperties, err error) {
	safeName := safeVHostName(vhost)
	file := filepath.Join(jp.dataDir, "vhosts", safeName, "exchanges", name+".json")

	data, err := os.ReadFile(file)
	if err != nil {
		return "", persistence.ExchangeProperties{}, err
	}

	var exchangeData JsonExchangeData

	if err := json.Unmarshal(data, &exchangeData); err != nil {
		return "", persistence.ExchangeProperties{}, err
	}
	return exchangeData.Type, exchangeData.Properties, nil
}

// DeleteExchange removes a single exchange JSON file
func (jp *JsonPersistence) DeleteExchangeMetadata(vhost, name string) error {
	safeName := safeVHostName(vhost)
	file := filepath.Join(jp.dataDir, "vhosts", safeName, "exchanges", name+".json")
	if err := os.Remove(file); err != nil {
		return err
	}
	return nil
}

// SaveQueueMetadata persists a single queue's metadata to a JSON file
func (jp *JsonPersistence) SaveQueueMetadata(vhost, name string, props persistence.QueueProperties) error {
	safeName := safeVHostName(vhost)
	dir := filepath.Join(jp.dataDir, "vhosts", safeName, "queues")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Update metadata while preserving messages
	queueData := JsonQueueData{
		Name:       name,
		Properties: props,
		Messages:   []JsonMessageData{}, // Preserve existing messages
	}

	// For JSON implementation, we need to preserve existing messages if they exist
	existingQueue, err := jp.loadQueueFile(vhost, name)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if existingQueue != nil && len(existingQueue.Messages) > 0 {
		queueData.Messages = existingQueue.Messages
	}

	file := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(queueData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

// LoadQueueMetadata loads a single queue's metadata from a JSON file (props)
func (jp *JsonPersistence) LoadQueueMetadata(vhost, name string) (props persistence.QueueProperties, err error) {
	queueData, err := jp.loadQueueFile(vhost, name)
	if err != nil {
		return persistence.QueueProperties{}, err
	}
	return queueData.Properties, nil
}

func (jp *JsonPersistence) SaveBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	// For JSON persistence, bindings are part of exchange metadata
	// So we can just load the exchange, update bindings, and save it back
	exchangeType, props, err := jp.LoadExchangeMetadata(vhost, exchange)
	if err != nil {
		return fmt.Errorf("failed to load exchange for binding: %v", err)
	}

	existingBindings, err := jp.LoadExchangeBindings(vhost, exchange)
	if err != nil {
		return fmt.Errorf("failed to load existing bindings: %v", err)
	}
	exchangeData := JsonExchangeData{
		Name:       exchange,
		Type:       exchangeType,
		Properties: props,
		Bindings:   existingBindings,
	}

	bindingMetadata := persistence.BindingData{
		QueueName:  queue,
		RoutingKey: routingKey,
		Arguments:  arguments,
	}
	// Deduplicate: avoid adding duplicate binding with same queue, routingKey, and arguments
	duplicate := false
	for _, b := range exchangeData.Bindings {
		if b.QueueName == bindingMetadata.QueueName && b.RoutingKey == bindingMetadata.RoutingKey {
			if equalArgs(b.Arguments, bindingMetadata.Arguments) {
				duplicate = true
				break
			}
		}
	}
	if !duplicate {
		exchangeData.Bindings = append(exchangeData.Bindings, bindingMetadata)
	}

	// Ensure directory exists before writing
	dir := filepath.Join(jp.dataDir, "vhosts", safeVHostName(vhost), "exchanges")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	file := filepath.Join(dir, exchange+".json")
	data, err := json.MarshalIndent(exchangeData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

func (jp *JsonPersistence) LoadExchangeBindings(vhost string, exchange string) ([]persistence.BindingData, error) {
	safeName := safeVHostName(vhost)
	file := filepath.Join(jp.dataDir, "vhosts", safeName, "exchanges", exchange+".json")

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var exchangeData JsonExchangeData

	if err := json.Unmarshal(data, &exchangeData); err != nil {
		return nil, err
	}
	return exchangeData.Bindings, nil
}

// DeleteBindingState deletes a binding between an exchange and a queue
func (jp *JsonPersistence) DeleteBindingState(vhost, exchange, queue, routingKey string, arguments map[string]any) error {
	exchangeType, props, err := jp.LoadExchangeMetadata(vhost, exchange)
	if err != nil {
		return fmt.Errorf("failed to load exchange for binding: %v", err)
	}

	existingBindings, err := jp.LoadExchangeBindings(vhost, exchange)
	if err != nil {
		return fmt.Errorf("failed to load existing bindings: %v", err)
	}
	// Filter out the binding to be deleted
	var updatedBindings []persistence.BindingData
	for _, b := range existingBindings {
		if !(b.QueueName == queue && b.RoutingKey == routingKey && equalArgs(b.Arguments, arguments)) {
			updatedBindings = append(updatedBindings, b)
		}
	}
	// remove the binding from the exchange data
	exchangeData := JsonExchangeData{
		Name:       exchange,
		Type:       exchangeType,
		Properties: props,
		Bindings:   updatedBindings,
	}

	// Ensure directory exists before writing (in case metadata was never saved)
	dir := filepath.Join(jp.dataDir, "vhosts", safeVHostName(vhost), "exchanges")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	file := filepath.Join(dir, exchange+".json")
	data, err := json.MarshalIndent(exchangeData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

func (jp *JsonPersistence) SaveMessage(vhost, queue, msgId string, msgBody []byte, msgProps persistence.MessageProperties) error {
	qData, err := jp.loadQueueFile(vhost, queue)
	if err != nil {
		return fmt.Errorf("queue not found: %v", err)
	}

	newMessage := JsonMessageData{
		ID:         msgId,
		Body:       msgBody,
		Properties: msgProps,
	}

	qData.Messages = append(qData.Messages, newMessage)

	// Save the complete queue data including messages
	return jp.saveQueueFile(vhost, queue, qData)
}

func (jp *JsonPersistence) LoadMessages(vhostName, queueName string) ([]persistence.Message, error) {
	qData, err := jp.loadQueueFile(vhostName, queueName)
	if err != nil {
		return nil, fmt.Errorf("queue not found: %v", err)
	}

	var messages []persistence.Message
	for _, msg := range qData.Messages {
		messages = append(messages, persistence.Message{
			ID:         msg.ID,
			Body:       msg.Body,
			Properties: msg.Properties,
		})
	}
	return messages, nil
}

func (jp *JsonPersistence) DeleteMessage(vhost, queue, msgId string) error {
	qData, err := jp.loadQueueFile(vhost, queue)
	if err != nil {
		return fmt.Errorf("queue not found: %v", err)
	}

	filtered := qData.Messages[:0]
	removed := false
	for _, msg := range qData.Messages {
		if msg.ID == msgId {
			removed = true
			continue
		}
		filtered = append(filtered, msg)
	}
	if !removed {
		return nil
	}
	qData.Messages = filtered

	return jp.saveQueueFile(vhost, queue, qData)
}

func (jp *JsonPersistence) LoadAllExchanges(vhost string) ([]persistence.ExchangeSnapshot, error) {
	safeName := safeVHostName(vhost)
	dir := filepath.Join(jp.dataDir, "vhosts", safeName, "exchanges")

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []persistence.ExchangeSnapshot{}, nil
	}
	if err != nil {
		return nil, err
	}

	var snapshots []persistence.ExchangeSnapshot
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		exchangeType, props, err := jp.LoadExchangeMetadata(vhost, name)
		if err != nil {
			continue // Skip corrupt files
		}
		bindings, _ := jp.LoadExchangeBindings(vhost, name)

		snapshots = append(snapshots, persistence.ExchangeSnapshot{
			Name:       name,
			Type:       exchangeType,
			Properties: props,
			Bindings:   bindings,
		})
	}
	return snapshots, nil
}

func (jp *JsonPersistence) LoadAllQueues(vhost string) ([]persistence.QueueSnapshot, error) {
	safeName := safeVHostName(vhost)
	dir := filepath.Join(jp.dataDir, "vhosts", safeName, "queues")

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []persistence.QueueSnapshot{}, nil
	}
	if err != nil {
		return nil, err
	}

	var snapshots []persistence.QueueSnapshot
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		queueData, err := jp.loadQueueFile(vhost, name)
		if err != nil {
			continue // Skip corrupt files
		}

		var messages []persistence.Message
		for _, msg := range queueData.Messages {
			messages = append(messages, persistence.Message{
				ID:         msg.ID,
				Body:       msg.Body,
				Properties: msg.Properties,
			})
		}

		snapshots = append(snapshots, persistence.QueueSnapshot{
			Name:       name,
			Properties: queueData.Properties,
			Messages:   messages,
		})
	}
	return snapshots, nil
}

/* ---- Private methods ---- */

// LoadQueue loads a single queue from a JSON file
func (jp *JsonPersistence) loadQueueFile(vhost, name string) (*JsonQueueData, error) {
	safeName := safeVHostName(vhost)
	file := filepath.Join(jp.dataDir, "vhosts", safeName, "queues", name+".json")

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var queueData JsonQueueData
	if err := json.Unmarshal(data, &queueData); err != nil {
		return nil, err
	}
	return &queueData, nil
}

func (jp *JsonPersistence) saveQueueFile(vhost, queueName string, queueData *JsonQueueData) error {
	safeName := safeVHostName(vhost)
	dir := filepath.Join(jp.dataDir, "vhosts", safeName, "queues")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file := filepath.Join(dir, queueName+".json")
	data, err := json.MarshalIndent(queueData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

// DeleteQueue removes a single queue JSON file
func (jp *JsonPersistence) DeleteQueueMetadata(vhostName, queueName string) error {
	safeName := safeVHostName(vhostName)
	file := filepath.Join(jp.dataDir, "vhosts", safeName, "queues", queueName+".json")
	if err := os.Remove(file); err != nil {
		return err
	}
	return nil
}
