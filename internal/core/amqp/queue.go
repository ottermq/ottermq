package amqp

import (
	"bytes"
	"fmt"

	"github.com/rs/zerolog/log"
)

type QueueDeclareMessage struct {
	QueueName  string
	Passive    bool
	Durable    bool
	Exclusive  bool
	AutoDelete bool
	NoWait     bool
	Arguments  map[string]interface{}
}

type QueueDeleteMessage struct {
	QueueName string
	IfUnused  bool
	NoWait    bool
}

type QueueBindMessage struct {
	Queue      string
	Exchange   string
	RoutingKey string
	NoWait     bool
	Arguments  map[string]interface{}
}

type QueueUnbindMessage struct {
	Queue      string
	Exchange   string
	RoutingKey string
	Arguments  map[string]interface{}
}

type QueuePurgeMessage struct {
	QueueName string
	NoWait    bool
}

func createQueueDeclareOkFrame(channel uint16, queueName string, messageCount, consumerCount uint32) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(QUEUE),
		MethodID: uint16(QUEUE_DECLARE_OK),
		Content: ContentList{
			KeyValuePairs: []KeyValue{
				{
					Key:   STRING_SHORT,
					Value: queueName,
				},
				{
					Key:   INT_LONG,
					Value: messageCount,
				},
				{
					Key:   INT_LONG,
					Value: consumerCount,
				},
			},
		},
	}.FormatMethodFrame()
	return frame
}

// createQueueAckFrame creates a method frame without additional content.
// To be used for QueueBindOk and QueueUnbindOk responses.
func createQueueAckFrame(channel, method uint16) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(QUEUE),
		MethodID: method,
		Content:  ContentList{},
	}.FormatMethodFrame()
	return frame
}

// createQueueCountAckFrame creates a method frame with the message count.
// To be used for QueueDeleteOk and QueuePurgeOk responses.
func createQueueCountAckFrame(channel, methodID uint16, messageCount uint32) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(QUEUE),
		MethodID: methodID,
		Content: ContentList{
			KeyValuePairs: []KeyValue{
				{
					Key:   INT_LONG,
					Value: messageCount,
				},
			},
		},
	}.FormatMethodFrame()
	return frame
}

func parseQueueMethod(methodID uint16, payload []byte) (any, error) {
	switch methodID {
	case uint16(QUEUE_DECLARE):
		log.Debug().Msg("Received QUEUE_DECLARE frame")
		return parseQueueDeclareFrame(payload)

	case uint16(QUEUE_DECLARE_OK):
		log.Debug().Msg("Received QUEUE_DECLARE_OK frame \n")
		// TODO: return the appropriate exception
		return nil, fmt.Errorf("server should not receive QUEUE_DECLARE_OK frames from clients")

	case uint16(QUEUE_BIND):
		log.Debug().Msg("Received QUEUE_BIND frame")
		return parseQueueBindFrame(payload)

	case uint16(QUEUE_PURGE):
		log.Debug().Msg("Received QUEUE_PURGE frame")
		return parseQueuePurgeFrame(payload)

	case uint16(QUEUE_DELETE):
		log.Debug().Msg("Received QUEUE_DELETE frame")
		return parseQueueDeleteFrame(payload)

	case uint16(QUEUE_UNBIND):
		log.Debug().Msg("Received QUEUE_UNBIND frame")
		return parseQueueUnbindFrame(payload)
	default:
		return nil, fmt.Errorf("unknown method ID: %d", methodID)
	}
}

// parseQueueDeclareFrame parses a QUEUE_DECLARE frame payload and returns a RequestMethodMessage.
func parseQueueDeclareFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}

	buf := bytes.NewReader(payload)
	reserverd1, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reserved1: %v", err)
	}
	if reserverd1 != 0 {
		return nil, fmt.Errorf("reserved1 must be 0")
	}
	queueName, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exchange name: %v", err)
	}
	octet, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read octet: %v", err)
	}
	flags := DecodeFlags(octet, []string{"passive", "durable", "exclusive", "autoDelete", "noWait"}, true)
	passive := flags["passive"]
	durable := flags["durable"]
	exclusive := flags["exclusive"]
	autoDelete := flags["autoDelete"]
	noWait := flags["noWait"]

	var arguments map[string]any
	if buf.Len() > 4 {

		argumentsStr, err := DecodeLongStr(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to decode arguments: %v", err)
		}
		arguments, err = DecodeTable([]byte(argumentsStr))
		if err != nil {
			return nil, fmt.Errorf("failed to read arguments: %v", err)
		}
	}
	msg := &QueueDeclareMessage{
		QueueName:  queueName,
		Passive:    passive,
		Durable:    durable,
		Exclusive:  exclusive,
		AutoDelete: autoDelete,
		NoWait:     noWait,
		Arguments:  arguments,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Debug().Interface("queue", msg).Msg("Queue formatted")
	return request, nil
}

// parseQueueBindFrame parses a QUEUE_BIND frame payload and returns a RequestMethodMessage.
func parseQueueBindFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}
	log.Printf("[DEBUG] Received QUEUE_BIND frame %x \n", payload)
	buf := bytes.NewReader(payload)
	reserverd1, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reserved1: %v", err)
	}
	if reserverd1 != 0 {
		return nil, fmt.Errorf("reserved1 must be 0")
	}
	queue, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode queue name: %v", err)
	}
	exchange, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exchange name: %v", err)
	}
	log.Printf("[DEBUG] Exchange name: %s\n", exchange)
	routingKey, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode routing key: %v", err)
	}
	octet, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read octet: %v", err)
	}
	flags := DecodeFlags(octet, []string{"noWait"}, true)
	noWait := flags["noWait"]
	arguments := make(map[string]any)
	if buf.Len() > 4 {
		argumentsStr, _ := DecodeLongStr(buf)
		arguments, err = DecodeTable([]byte(argumentsStr))
		if err != nil {
			return nil, fmt.Errorf("failed to read arguments: %v", err)
		}
	}
	msg := &QueueBindMessage{
		Queue:      queue,
		Exchange:   exchange,
		RoutingKey: routingKey,
		NoWait:     noWait,
		Arguments:  arguments,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Debug().Interface("queue", msg).Msg("Queue formatted")
	return request, nil
}

// parseQueueUnbindFrame parses a QUEUE_UNBIND frame payload and returns a RequestMethodMessage.
func parseQueueUnbindFrame(payload []byte) (any, error) {
	// Expected fields:
	// reserved1(shortint),
	// queue (short str),
	// exchange (short str),
	// routing key (short str),
	// arguments (table)
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}
	buf := bytes.NewReader(payload)
	_, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reserved1: %v", err)
	}
	queue, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode queue name: %v", err)
	}
	exchange, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exchange name: %v", err)
	}
	routingKey, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode routing key: %v", err)
	}
	arguments := make(map[string]any)
	if buf.Len() > 4 {
		argumentsStr, _ := DecodeLongStr(buf)
		arguments, err = DecodeTable([]byte(argumentsStr))
		if err != nil {
			return nil, fmt.Errorf("failed to read arguments: %v", err)
		}
	}
	msg := &QueueUnbindMessage{
		Queue:      queue,
		Exchange:   exchange,
		RoutingKey: routingKey,
		Arguments:  arguments,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Debug().Interface("queue", msg).Msg("Queue formatted")
	return request, nil

}

func parseQueuePurgeFrame(payload []byte) (any, error) {
	// 2 (reserved1) => short int = 2 bytes
	// 1+ (queue name) => short str = 1 (length) + 0+ bytes
	// No-wait (bit) => octet = 1 byte
	if len(payload) < 4 {
		return nil, fmt.Errorf("payload too short")
	}
	buf := bytes.NewReader(payload)
	_, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reserved1: %v", err)
	}
	queueName, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode queue name: %v", err)
	}
	octet, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read octet: %v", err)
	}
	flags := DecodeFlags(octet, []string{"noWait"}, true)
	noWait := flags["noWait"]

	msg := &QueuePurgeMessage{
		QueueName: queueName,
		NoWait:    noWait,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Debug().Interface("queue", msg).Msg("Queue formatted")
	return request, nil
}

// parseQueueDeleteFrame parses a QUEUE_DELETE frame payload and returns a RequestMethodMessage.
func parseQueueDeleteFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}
	log.Printf("[DEBUG] Received QUEUE_DELETE frame %x \n", payload)

	buf := bytes.NewReader(payload)
	reserverd1, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reserved1: %v", err)
	}
	if reserverd1 != 0 {
		return nil, fmt.Errorf("reserved1 must be 0")
	}
	exchangeName, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exchange name: %v", err)
	}
	octet, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read octet: %v", err)
	}
	flags := DecodeFlags(octet, []string{"ifUnused", "noWait"}, true)
	ifUnused := flags["ifUnused"]
	noWait := flags["noWait"]

	msg := &QueueDeleteMessage{
		QueueName: exchangeName,
		IfUnused:  ifUnused,
		NoWait:    noWait,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Debug().Interface("queue", msg).Msg("Queue formatted")
	return request, nil
}
