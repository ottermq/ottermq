package amqp

import (
	"encoding/binary"
	"fmt"

	"github.com/rs/zerolog/log"
)

func parseFrame(frame []byte) (any, error) {
	if len(frame) < 7 {
		return nil, fmt.Errorf("frame too short")
	}

	frameType := frame[0]
	channel := binary.BigEndian.Uint16(frame[1:3])
	payloadSize := binary.BigEndian.Uint32(frame[3:7])
	if len(frame) < int(7+payloadSize) {
		return nil, fmt.Errorf("frame too short")
	}
	payload := frame[7:]

	switch frameType {
	case byte(TYPE_METHOD):
		log.Trace().Uint16("channel", channel).Msg("Received METHOD frame")
		request, err := parseMethodFrame(channel, payload)
		if err != nil {
			return nil, fmt.Errorf("failed to parse method frame: %v", err)
		}
		return request, nil

	case byte(TYPE_HEADER):
		log.Trace().Uint16("channel", channel).Msg("Received HEADER frame")
		return parseHeaderFrame(channel, payloadSize, payload)

	case byte(TYPE_BODY):
		log.Trace().Uint16("channel", channel).Msg("Received BODY frame")
		return parseBodyFrame(payloadSize, payload)

	case byte(TYPE_HEARTBEAT):
		log.Trace().Msg("Received HEARTBEAT frame")
		return &Heartbeat{}, nil

	default:
		log.Trace().Bytes("frame", frame).Msg("Received unknown frame")
		return nil, fmt.Errorf("unknown frame type: %d", frameType)
	}
}

func parseMethodFrame(channel uint16, payload []byte) (*ChannelState, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("payload too short")
	}

	classID := binary.BigEndian.Uint16(payload[0:2])
	methodID := binary.BigEndian.Uint16(payload[2:4])
	methodPayload := payload[4:]

	switch classID {
	case uint16(CONNECTION):
		log.Trace().Msgf("Received CONNECTION frame on channel %d\n", channel)
		startOkFrame, err := parseConnectionMethod(methodID, methodPayload)
		if err != nil {
			return nil, err
		}

		request := &RequestMethodMessage{
			Channel:  channel,
			ClassID:  classID,
			MethodID: methodID,
			Content:  startOkFrame,
		}
		state := &ChannelState{
			MethodFrame: request,
		}
		return state, nil

	case uint16(CHANNEL):
		log.Trace().Msgf("Received CHANNEL frame on channel %d\n", channel)
		request, err := parseChannelMethod(methodID, methodPayload)
		if err != nil {
			return nil, err
		}
		if request != nil {
			msg, ok := request.(*RequestMethodMessage)
			if ok {
				msg.Channel = channel
				msg.ClassID = classID
				msg.MethodID = methodID
				state := &ChannelState{
					MethodFrame: msg,
				}
				return state, nil
			}
		}
		return nil, nil

	case uint16(EXCHANGE):
		log.Trace().Msgf("Received EXCHANGE frame on channel %d\n", channel)
		request, err := parseExchangeMethod(methodID, methodPayload)
		if err != nil {
			return nil, err
		}
		if request != nil {
			msg, ok := request.(*RequestMethodMessage)
			if ok {
				msg.Channel = channel
				msg.ClassID = classID
				msg.MethodID = methodID
				state := &ChannelState{
					MethodFrame: msg,
				}
				return state, nil
			}
		}
		return nil, nil

	case uint16(QUEUE):
		log.Trace().Msgf("Received QUEUE frame on channel %d\n", channel)
		request, err := parseQueueMethod(methodID, methodPayload)
		if err != nil {
			return nil, err
		}
		if request != nil {
			msg, ok := request.(*RequestMethodMessage)
			if ok {
				msg.Channel = channel
				msg.ClassID = classID
				msg.MethodID = methodID
				state := &ChannelState{
					MethodFrame: msg,
				}
				return state, nil
			}
		}
		return nil, nil

	case uint16(BASIC):
		log.Debug().Uint16("channel", channel).Msg("Received BASIC frame")
		request, err := parseBasicMethod(methodID, methodPayload)
		if err != nil {
			// The raised exception (connection type):
			// (501 - "frame error"),
			// (502 - "syntax error"),
			// (503 - "command invalid"),
			// (504 - "channel error"), -- shall be handled at a higher level
			// (505 - "unexpected frame"),
			// (506 - "resource error"), -- shall be handled at a higher level
			// (530 - "not allowed"), -- shall be handled at a higher level
			// (540 - "not implemented"),
			// (541 - "internal error"),
			//
			return nil, err
		}
		if request != nil {
			msg, ok := request.(*RequestMethodMessage)
			if ok {
				msg.Channel = channel
				msg.ClassID = classID
				msg.MethodID = methodID
				state := &ChannelState{
					MethodFrame: msg,
				}
				return state, nil
			}
		}
		return nil, nil

	case uint16(TX):
		log.Debug().Uint16("channel", channel).Msg("Received TX frame")
		request, err := parseTxMethod(methodID, methodPayload)
		if err != nil {
			return nil, err
		}
		if request != nil {
			msg, ok := request.(*RequestMethodMessage)
			if ok {
				msg.Channel = channel
				msg.ClassID = classID
				msg.MethodID = methodID
				state := &ChannelState{
					MethodFrame: msg,
				}
				return state, nil
			}
		}
		return nil, nil

	default:
		log.Warn().Uint16("class_id", classID).Msg("Unknown class ID")
		return nil, fmt.Errorf("unknown class ID: %d", classID)
	}
}

func parseHeaderFrame(channel uint16, payloadSize uint32, payload []byte) (*ChannelState, error) {

	if len(payload) < int(payloadSize) {
		return nil, fmt.Errorf("payload too short")
	}
	log.Trace().Uint32("payload_size", payloadSize).Msg("Received payload size")

	classID := binary.BigEndian.Uint16(payload[0:2])

	switch classID {

	case uint16(BASIC):
		log.Trace().Uint16("channel", channel).Msg("Received BASIC HEADER frame")
		request, err := parseBasicHeader(payload)
		if err != nil {
			return nil, err
		}
		if request != nil {
			request.Channel = channel
			state := &ChannelState{
				HeaderFrame: request,
			}
			return state, nil
		}
		return nil, nil

	default:
		log.Warn().Uint16("class_id", classID).Msg("Unknown class ID")
		return nil, fmt.Errorf("unknown class ID: %d", classID)
	}
}

func parseBodyFrame(payloadSize uint32, payload []byte) (*ChannelState, error) {

	if len(payload) < int(payloadSize) {
		return nil, fmt.Errorf("payload too short")
	}
	log.Trace().Uint32("payload_size", payloadSize).Msg("Received payload size")

	state := &ChannelState{
		Body: payload,
	}
	return state, nil

}
