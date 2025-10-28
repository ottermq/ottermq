package amqp

import (
	"bytes"
	"encoding/binary"
	"net"

	"github.com/rs/zerolog/log"
)

// ClassID: 10 (connection) |
// MethodID: 40 (close)
type ConnectionCloseMessage struct {
	ReplyCode uint16
	ReplyText string
	ClassID   uint16
	MethodID  uint16
}

// ClassID: 10 (connection) |
// MethodID: 41 (close-Ok)
type ConnectionCloseOkMessage struct {
}

func sendHeartbeat(conn net.Conn) error {
	heartbeatFrame := createHeartbeatFrame()
	return sendFrame(conn, heartbeatFrame)
}

func createCloseFrame(channel, replyCode, classID, methodID, closeClassID, closeClassMethod uint16, replyText string) []byte {
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
		ClassID:  closeClassID,
		MethodID: closeClassMethod,
		Content:  content,
	}.FormatMethodFrame()
	return frame
}

func createConnectionCloseOkFrame(request *RequestMethodMessage) []byte {
	frame := ResponseMethodMessage{
		Channel:  request.Channel,
		ClassID:  uint16(CONNECTION),
		MethodID: uint16(CONNECTION_CLOSE_OK),
		Content:  ContentList{},
	}.FormatMethodFrame()
	return frame
}

func createConnectionStartFrame(configurations *map[string]any) []byte {
	var payloadBuf bytes.Buffer
	channelNum := uint16(0)
	classID := CONNECTION
	methodID := CONNECTION_START

	payloadBuf.WriteByte(0) // version-major
	payloadBuf.WriteByte(9) // version-minor

	serverPropsRaw, ok := (*configurations)["serverProperties"]
	if !ok {
		log.Fatal().Msg("serverProperties not found in configurations")
	}
	serverProperties, ok := serverPropsRaw.(map[string]any)
	if !ok {
		log.Fatal().Msg("serverProperties is not a map[string]any")
	}
	encodedProperties := EncodeTable(serverProperties)

	EncodeLongStr(&payloadBuf, encodedProperties)

	// Extract mechanisms
	mechanismsRaw, ok := (*configurations)["mechanisms"]
	if !ok {
		log.Fatal().Msg("mechanisms not found in configurations")
	}
	mechanismsSlice, ok := mechanismsRaw.([]string)
	if !ok || len(mechanismsSlice) == 0 {
		log.Fatal().Msg("mechanisms is not a non-empty []string")
	}
	EncodeLongStr(&payloadBuf, []byte(mechanismsSlice[0]))

	// Extract locales
	localesRaw, ok := (*configurations)["locales"]
	if !ok {
		log.Fatal().Msg("locales not found in configurations")
	}
	localesSlice, ok := localesRaw.([]string)
	if !ok || len(localesSlice) == 0 {
		log.Fatal().Msg("locales is not a non-empty []string")
	}
	EncodeLongStr(&payloadBuf, []byte(localesSlice[0]))

	frame := formatMethodFrame(channelNum, classID, methodID, payloadBuf.Bytes())
	log.Trace().Msgf("Sending CONNECTION_START frame: %v", frame)
	return frame
}

func createConnectionTuneFrame(tune *ConnectionTune) []byte {
	var payloadBuf bytes.Buffer
	channelNum := uint16(0)
	classID := CONNECTION
	methodID := CONNECTION_TUNE

	if err := binary.Write(&payloadBuf, binary.BigEndian, tune.ChannelMax); err != nil {
		return nil
	}
	if err := binary.Write(&payloadBuf, binary.BigEndian, tune.FrameMax); err != nil {
		return nil
	}
	if err := binary.Write(&payloadBuf, binary.BigEndian, tune.Heartbeat); err != nil {
		return nil
	}

	frame := formatMethodFrame(channelNum, classID, methodID, payloadBuf.Bytes())
	return frame
}

func createConnectionTuneOkFrame(tune *ConnectionTune) []byte {
	var payloadBuf bytes.Buffer
	channelNum := uint16(0)
	classID := CONNECTION
	methodID := CONNECTION_TUNE_OK

	if err := binary.Write(&payloadBuf, binary.BigEndian, tune.ChannelMax); err != nil {
		return nil
	}
	if err := binary.Write(&payloadBuf, binary.BigEndian, tune.FrameMax); err != nil {
		return nil
	}
	if err := binary.Write(&payloadBuf, binary.BigEndian, tune.Heartbeat); err != nil {
		return nil
	}

	frame := formatMethodFrame(channelNum, classID, methodID, payloadBuf.Bytes())
	return frame
}

func createConnectionOpenOkFrame() []byte {
	var payloadBuf bytes.Buffer
	channelNum := uint16(0)
	classID := CONNECTION
	methodID := CONNECTION_OPEN_OK

	// Reserved-1 (bit) - set to 0
	payloadBuf.WriteByte(0)

	frame := formatMethodFrame(channelNum, classID, methodID, payloadBuf.Bytes())
	return frame
}

func fineTune(tune *ConnectionTune) *ConnectionTune {
	// TODO: get values from config
	tune.ChannelMax = getSmalestShortInt(2047, tune.ChannelMax)
	tune.FrameMax = getSmalestLongInt(131072, tune.FrameMax)
	tune.Heartbeat = getSmalestShortInt(10, tune.Heartbeat)

	return tune
}

func createHeartbeatFrame() []byte {
	frame := make([]byte, 8)
	frame[0] = byte(TYPE_HEARTBEAT)
	binary.BigEndian.PutUint16(frame[1:3], 0)
	binary.BigEndian.PutUint32(frame[3:7], 0)
	frame[7] = FRAME_END
	return frame
}
