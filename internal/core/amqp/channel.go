package amqp

import (
	"bytes"
	"fmt"

	"github.com/rs/zerolog/log"
)

type ChannelCloseMessage struct {
	ReplyCode uint16
	ReplyText string
	ClassID   uint16
	MethodID  uint16
}

type ChannelOpenMessage struct {
}

func createChannelOpenOkFrame(request *RequestMethodMessage) []byte {
	frame := ResponseMethodMessage{
		Channel:  request.Channel,
		ClassID:  uint16(CHANNEL),
		MethodID: uint16(CHANNEL_OPEN_OK),
		Content: ContentList{
			KeyValuePairs: []KeyValue{
				{
					Key:   INT_LONG,
					Value: uint32(0),
				},
			},
		},
	}.FormatMethodFrame()
	return frame
}

func createChannelCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte {
	replyCodeKv := KeyValue{
		Key:   INT_SHORT,
		Value: replyCode,
	}
	replyTextKv := KeyValue{
		Key:   STRING_SHORT,
		Value: replyText,
	}
	classIDKv := KeyValue{
		Key:   INT_SHORT,
		Value: classID,
	}
	methodIDKv := KeyValue{
		Key:   INT_SHORT,
		Value: methodID,
	}
	content := ContentList{
		KeyValuePairs: []KeyValue{replyCodeKv, replyTextKv, classIDKv, methodIDKv},
	}
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(CHANNEL),
		MethodID: uint16(CHANNEL_CLOSE),
		Content:  content,
	}.FormatMethodFrame()
	return frame
}

func createChannelCloseOkFrame(channel uint16) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(CHANNEL),
		MethodID: uint16(CHANNEL_CLOSE_OK),
		Content:  ContentList{},
	}.FormatMethodFrame()
	return frame
}

func parseChannelMethod(methodID uint16, payload []byte) (interface{}, error) {
	switch methodID {
	case uint16(CHANNEL_OPEN):
		log.Debug().Msg("Received CHANNEL_OPEN frame")
		return parseChannelOpenFrame(payload)

	case uint16(CHANNEL_OPEN_OK):
		log.Debug().Msg("Received CHANNEL_OPEN_OK frame")
		return parseChannelOpenOkFrame(payload)

	case uint16(CHANNEL_CLOSE):
		log.Debug().Msg("Received CHANNEL_CLOSE frame")
		return parseChannelCloseFrame(payload)

	case uint16(CHANNEL_CLOSE_OK):
		log.Debug().Msg("Received CHANNEL_CLOSE_OK frame")
		return parseChannelCloseOkFrame(payload)

	default:
		return nil, fmt.Errorf("unknown method ID: %d", methodID)
	}
}

func parseChannelOpenFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("payload too short")
	}
	request := &RequestMethodMessage{
		Content: nil,
	}

	return request, nil
}

func parseChannelOpenOkFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("payload too short")
	}
	request := &RequestMethodMessage{
		Content: nil,
	}

	return request, nil
}

func parseChannelCloseFrame(payload []byte) (*RequestMethodMessage, error) {
	log.Trace().Msgf("Received CHANNEL_CLOSE frame: %x \n", payload)
	if len(payload) < 6 {
		return nil, fmt.Errorf("frame too short")
	}

	buf := bytes.NewReader(payload)
	replyCode, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed decoding reply code: %v", err)
	}
	replyText, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed decoding reply text: %v", err)
	}
	classID, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed decoding class id: %v", err)
	}
	methodID, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed decoding method id: %v", err)
	}

	msg := &ChannelCloseMessage{
		ReplyCode: replyCode,
		ReplyText: replyText,
		ClassID:   classID,
		MethodID:  methodID,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	return request, nil
}

func parseChannelCloseOkFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) > 1 {
		return nil, fmt.Errorf("unexxpected payload length")
	}
	request := &RequestMethodMessage{
		Content: nil,
	}

	return request, nil
}
