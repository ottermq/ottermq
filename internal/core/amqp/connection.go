package amqp

import (
	"bytes"
	"encoding/binary"
	"fmt"
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

func createConnectionCloseFrame(channel, replyCode, classID, methodID uint16, replyText string) []byte {
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
		ClassID:  uint16(CONNECTION),
		MethodID: uint16(CONNECTION_CLOSE),
		Content:  content,
	}.FormatMethodFrame()
	return frame
}

func createConnectionCloseOkFrame(channel uint16) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
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

func parseConnectionMethod(methodID uint16, payload []byte) (any, error) {
	switch methodID {
	case uint16(CONNECTION_START):
		log.Debug().Msg("Received CONNECTION_START frame")
		return parseConnectionStartFrame(payload)

	case uint16(CONNECTION_START_OK):
		log.Trace().Bytes("payload", payload).Msg("Received connection.start-ok")
		return parseConnectionStartOkFrame(payload)

	case uint16(CONNECTION_TUNE):
		log.Debug().Msg("Received CONNECTION_TUNE frame")
		tuneResponse, err := parseConnectionTuneFrame(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to parse connection.tune frame: %v", err)
		}
		tuneOkRequest := createConnectionTuneOkFrame(fineTune(tuneResponse))
		return tuneOkRequest, nil

	case uint16(CONNECTION_TUNE_OK):
		log.Debug().Msg("Received CONNECTION_TUNE_OK frame")
		return parseConnectionTuneOkFrame(payload)

	case uint16(CONNECTION_OPEN):
		log.Debug().Msg("Received CONNECTION_OPEN frame")
		return parseConnectionOpenFrame(payload)

	case uint16(CONNECTION_OPEN_OK):
		log.Debug().Msg("Received CONNECTION_OPEN_OK frame")
		return parseConnectionOpenOkFrame(payload)

	case uint16(CONNECTION_CLOSE):
		request, err := parseConnectionCloseFrame(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to parse connection.close frame: %v", err)
		}
		request.MethodID = methodID
		content, ok := request.Content.(*ConnectionCloseMessage)
		if !ok {
			return nil, fmt.Errorf("invalid message content type")
		}
		log.Debug().Msgf("Received CONNECTION_CLOSE: (%d) '%s'", content.ReplyCode, content.ReplyText)
		return request, nil

	case uint16(CONNECTION_CLOSE_OK):
		request, err := parseConnectionCloseOkFrame(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CONNECTION_CLOSE_OK frame: %v", err)
		}
		request.MethodID = methodID
		log.Debug().Msg("Received CONNECTION_CLOSE_OK")
		return request, nil

	default:
		return nil, fmt.Errorf("unknown method ID: %d", methodID)
	}
}

func parseConnectionCloseFrame(payload []byte) (*RequestMethodMessage, error) {
	if len(payload) < 12 {
		return nil, fmt.Errorf("frame too short")
	}

	index := uint16(0)
	replyCode := binary.BigEndian.Uint16(payload[index : index+2])
	index += 2
	replyTextLen := payload[index]
	index++
	replyText := string(payload[index : index+uint16(replyTextLen)])
	index += uint16(replyTextLen)
	classID := binary.BigEndian.Uint16(payload[index : index+2])
	index += 2
	methodID := binary.BigEndian.Uint16(payload[index : index+2])
	msg := &ConnectionCloseMessage{
		ReplyCode: replyCode,
		ReplyText: replyText,
		ClassID:   classID,
		MethodID:  methodID,
	}
	request := &RequestMethodMessage{
		Content: msg,
	}
	log.Trace().Msgf("connection close request: %+v\n", request)
	return request, nil
}

func parseConnectionCloseOkFrame(payload []byte) (*RequestMethodMessage, error) {
	return &RequestMethodMessage{Content: payload}, nil
}

func parseConnectionOpenFrame(payload []byte) (interface{}, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("payload too short")
	}
	buf := bytes.NewReader(payload)
	virtualHost, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode virtual host: %v", err)
	}
	return &ConnectionOpen{
		VirtualHost: virtualHost,
	}, nil
}

func parseConnectionOpenOkFrame(payload []byte) (interface{}, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("payload too short")
	}
	// buf := bytes.NewReader(payload)
	return nil, nil
}

func parseConnectionTuneFrame(payload []byte) (*ConnectionTune, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}
	buf := bytes.NewReader(payload)
	channelMax, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode channel max: %v", err)
	}

	frameMax, err := DecodeLongInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode frame max: %v", err)
	}
	heartbeat, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode heartbeat: %v", err)
	}
	return &ConnectionTune{
		ChannelMax: channelMax,
		FrameMax:   frameMax,
		Heartbeat:  heartbeat,
	}, nil
}

func parseConnectionTuneOkFrame(payload []byte) (interface{}, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}
	buf := bytes.NewReader(payload)
	channelMax, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode channel max: %v", err)
	}

	frameMax, err := DecodeLongInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode frame max: %v", err)
	}
	heartbeat, err := DecodeShortInt(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode heartbeat: %v", err)
	}
	return &ConnectionTune{
		ChannelMax: channelMax,
		FrameMax:   frameMax,
		Heartbeat:  heartbeat,
	}, nil
}

func parseConnectionStartFrame(payload []byte) (*ConnectionStartFrame, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}

	buf := bytes.NewReader(payload)

	// Decode version-major and version-minor
	versionMajor, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to decode version-major: %v", err)
	}
	log.Trace().Msgf("Version Major: %d\n", versionMajor)

	versionMinor, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to decode version-minor: %v", err)
	}
	log.Trace().Msgf("Version Minor: %d\n", versionMinor)

	/***Server Properties*/
	serverPropertiesStr, err := DecodeLongStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode client properties: %v", err)
	}
	serverProperties, err := DecodeTable([]byte(serverPropertiesStr))
	if err != nil {
		return nil, fmt.Errorf("failed to decode client properties: %v", err)
	}
	log.Trace().Msgf("Server Properties: %+v\n", serverProperties)

	/***Mechanisms*/
	mechanismsStr, err := DecodeLongStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode mechanism: %v", err)
	}
	log.Trace().Msgf("Mechanisms: %s\n", mechanismsStr)

	/***Locales*/
	localesStr, err := DecodeLongStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode locale: %v", err)
	}
	log.Trace().Msgf("Locales: %s\n", localesStr)

	return &ConnectionStartFrame{
		VersionMajor:     versionMajor,
		VersionMinor:     versionMinor,
		ServerProperties: serverProperties,
		Mechanisms:       mechanismsStr,
		Locales:          localesStr,
	}, nil
}

func parseConnectionStartOkFrame(payload []byte) (*ConnectionStartOk, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("payload too short")
	}

	buf := bytes.NewReader(payload)

	clientPropertiesStr, err := DecodeLongStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode client properties: %v", err)
	}

	clientProperties, err := DecodeTable([]byte(clientPropertiesStr))
	if err != nil {
		return nil, fmt.Errorf("failed to decode client properties: %v", err)
	}

	mechanism, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode mechanism: %v", err)
	}

	if mechanism != "PLAIN" {
		return nil, fmt.Errorf("unsupported mechanism: %s", mechanism)
	}

	security, err := DecodeSecurityPlain(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode security: %v", err)
	}

	locale, err := DecodeShortStr(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode locale: %v", err)
	}

	return &ConnectionStartOk{
		ClientProperties: clientProperties,
		Mechanism:        mechanism,
		Response:         security,
		Locale:           locale,
	}, nil
}
