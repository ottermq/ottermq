package amqp

import (
	"bytes"
	"fmt"

	"github.com/rs/zerolog/log"
)

type ExchangeDeclareMessage struct {
	ExchangeName string
	ExchangeType string
	Passive      bool
	Durable      bool
	AutoDelete   bool
	Internal     bool
	NoWait       bool
	Arguments    map[string]interface{}
}

type ExchangeDeleteMessage struct {
	ExchangeName string
	IfUnused     bool
	NoWait       bool
}

func createExchangeDeclareFrameOk(channel uint16) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(EXCHANGE),
		MethodID: uint16(EXCHANGE_DECLARE_OK),
		Content:  ContentList{},
	}.FormatMethodFrame()
	return frame
}

func createExchangeDeleteFrame(channel uint16) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(EXCHANGE),
		MethodID: uint16(EXCHANGE_DELETE_OK),
		Content:  ContentList{},
	}.FormatMethodFrame()
	return frame
}

func parseExchangeMethod(methodID uint16, payload []byte) (interface{}, error) {
	switch methodID {
	case uint16(EXCHANGE_DECLARE):
		log.Debug().Msg("Received EXCHANGE_DECLARE frame \n")
		return parseExchangeDeclareFrame(payload)
	case uint16(EXCHANGE_DELETE):
		log.Debug().Msg("Received EXCHANGE_DELETE frame \n")
		return parseExchangeDeleteFrame(payload)

	default:
		return nil, fmt.Errorf("unknown method ID: %d", methodID)
	}
}

// Fields:
// 0-1: reserved short int
// 2: exchange name - length (short)
// 3: type - (string)
// 4: passive - (bit)
// 5: durable - (bit)
// 6: reserved 2
// 7: reserved 3
// 8: no-wait - (bit)
// 9: arguments - (table)
func parseExchangeDeclareFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}

	buf := bytes.NewReader(payload)
	reserved1, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reserved1: %v", err)
	}
	if reserved1 != 0 {
		return nil, fmt.Errorf("reserved1 must be 0")
	}
	exchangeName, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exchange name: %v", err)
	}
	exchangeType, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exchange type: %v", err)
	}
	octet, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read octet: %v", err)
	}
	flags := DecodeFlags(octet, []string{"passive", "durable", "autoDelete", "internal", "noWait"}, true)
	passive := flags["passive"]
	durable := flags["durable"]
	autoDelete := flags["autoDelete"]
	internal := flags["internal"]
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
	msg := &ExchangeDeclareMessage{
		ExchangeName: exchangeName,
		ExchangeType: exchangeType,
		Passive:      passive,
		Durable:      durable,
		AutoDelete:   autoDelete,
		Internal:     internal,
		NoWait:       noWait,
		Arguments:    arguments,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Trace().Interface("msg", msg).Msg("exchange.declare formatted")
	return request, nil
}

// Fields:
// 0-1: reserved short int
// 2: exchange name - length (short)
// 3: if-unused - (bit)
// 4: no-wait - (bit)
func parseExchangeDeleteFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}
	log.Printf("[DEBUG] Received EXCHANGE_DELETE frame %x \n", payload)

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

	msg := &ExchangeDeleteMessage{
		ExchangeName: exchangeName,
		IfUnused:     ifUnused,
		NoWait:       noWait,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Trace().Interface("msg", msg).Msg("exchange.delete formatted")
	return request, nil
}
