package amqp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/ottermq/ottermq/internal/persistdb"
)

func handshake(configurations *map[string]any, conn net.Conn, connCtx context.Context) (*ConnectionInfo, error) {
	// gets the protocol header sent by the client
	clientHeader, err := readProtocolHeader(conn)
	if err != nil {
		return nil, err
	}
	log.Trace().Str("header", fmt.Sprintf("%x", clientHeader)).Msg("Handshake - Received")
	protocol := (*configurations)["protocol"].(string)
	protocolHeader, err := buildProtocolHeader(protocol)
	if err != nil {
		msg := "error parsing protocol"
		log.Error().Str("error", msg).Msg("Handshake error")
		return nil, fmt.Errorf("%s", msg)
	}
	if !bytes.Equal(clientHeader, protocolHeader) {
		err := sendProtocolHeader(conn, protocolHeader)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("bad protocol: %x (%s -> %s)", clientHeader, conn.RemoteAddr().String(), conn.LocalAddr().String())
	}
	log.Debug().Str("remote", conn.RemoteAddr().String()).Str("local", conn.LocalAddr().String()).Msg("Handshake - Accepting AMQP connection")

	/** connection.start **/
	// send connection.start frame
	startFrame := createConnectionStartFrame(configurations)
	if err := sendFrame(conn, startFrame); err != nil {
		return nil, err
	}

	/** R connection.start-ok -> W connection.tune **/
	// read connecion.start-ok frame
	frame, err := readFrame(conn)
	if err != nil {
		return nil, err
	}
	log.Trace().Msgf("Handshake - Received: %x\n", frame)
	response, err := parseFrame(frame)
	if err != nil {
		return nil, err
	}
	state, ok := response.(*ChannelState)
	if !ok {
		return nil, fmt.Errorf("type assertion ChannelState failed")
	}
	if state.MethodFrame == nil {
		return nil, fmt.Errorf("methodFrame is empty")
	}
	startOkFrame, ok := state.MethodFrame.Content.(*ConnectionStartOk)
	if !ok {
		return nil, fmt.Errorf("type assertion ConnectionStartOkFrame failed")
	}
	if startOkFrame == nil {
		return nil, fmt.Errorf("type assertion ConnectionStartOkFrame failed")
	}

	err = processStartOkContent(configurations, startOkFrame)
	if err != nil {
		return nil, err
	}
	log.Debug().Str("remote", conn.RemoteAddr().String()).Str("local", conn.LocalAddr().String()).Msg("Handshake - Started")

	heartbeat, _ := (*configurations)["heartbeatInterval"].(uint16)
	frameMax, _ := (*configurations)["frameMax"].(uint32)
	channelMax, _ := (*configurations)["channelMax"].(uint16)

	tune := &ConnectionTune{
		ChannelMax: uint16(channelMax),
		FrameMax:   uint32(frameMax),
		Heartbeat:  uint16(heartbeat),
	}
	// create tune frame
	tuneFrame := createConnectionTuneFrame(tune)
	if err := sendFrame(conn, tuneFrame); err != nil {
		return nil, err
	}

	// read connection.tune-ok frame
	frame, err = readFrame(conn)
	if err != nil {
		return nil, err
	}
	response, err = parseFrame(frame)
	if err != nil {
		return nil, err
	}
	state, ok = response.(*ChannelState)
	if !ok {
		return nil, fmt.Errorf("type assertion ChannelState failed")
	}
	if state.MethodFrame == nil {
		return nil, fmt.Errorf("MethodFrame is empty")
	}
	tuneOkFrame := state.MethodFrame.Content.(*ConnectionTune)
	if tuneOkFrame == nil {
		return nil, fmt.Errorf("type assertion ConnectionTuneOkFrame failed")
	}

	(*configurations)["heartbeatInterval"] = tuneOkFrame.Heartbeat
	(*configurations)["frameMax"] = tuneOkFrame.FrameMax
	(*configurations)["channelMax"] = tuneOkFrame.ChannelMax

	config := NewAmqpClientConfig(configurations)
	connCtx, cancel := context.WithCancel(connCtx)
	client := NewAmqpClient(conn, config, connCtx, cancel)
	log.Debug().Msg("Connection successfully tuned")

	client.StartHeartbeat()

	// read connection.open frame
	frame, err = readFrame(conn)
	if err != nil {
		return nil, err
	}

	response, err = parseFrame(frame)
	if err != nil {
		return nil, err
	}
	state, ok = response.(*ChannelState)
	if !ok {
		return nil, fmt.Errorf("type assertion ConnectionOpen failed")
	}
	if state.MethodFrame == nil {
		return nil, fmt.Errorf("methodFrame is empty")
	}

	openFrame, _ := state.MethodFrame.Content.(*ConnectionOpen)
	if openFrame == nil {
		return nil, fmt.Errorf("type assertion ConnectionOpenFrame failed")
	}
	VHostName := openFrame.VirtualHost

	// Enforce vhost membership: check that the authenticated user is allowed on this vhost.
	username, _ := (*configurations)["username"].(string)
	if username != "" {
		ok, err := persistdb.HasVHostAccess(username, VHostName)
		if err != nil || !ok {
			return nil, fmt.Errorf("access to vhost '%s' denied for user '%s'", VHostName, username)
		}
	}

	connInfo := NewConnectionInfo(VHostName)
	connInfo.Client = client

	//send connection.open-ok frame
	openOkFrame := createConnectionOpenOkFrame()
	if err := sendFrame(conn, openOkFrame); err != nil {
		return nil, err
	}
	log.Debug().Str("remote", conn.RemoteAddr().String()).Str("local", conn.LocalAddr().String()).Msg("Handshake - Opened")
	log.Debug().Msg("Handshake Completed")

	return connInfo, nil
}

func processStartOkContent(configurations *map[string]any, startOkFrame *ConnectionStartOk) error {
	mechanism := startOkFrame.Mechanism
	if mechanism != "PLAIN" {
		return fmt.Errorf("mechanism invalid or %s not suported", mechanism)
	}
	// parse username and password from startOkFrame.Response
	log.Printf("Response: '%s'\n", startOkFrame.Response)
	credentials := strings.Split(strings.Trim(startOkFrame.Response, " "), " ")
	log.Debug().Str("username", credentials[0]).Msg("Credentials parsed")
	if len(credentials) != 2 {
		return fmt.Errorf("failed to parse credentials: invalid format")
	}
	username := credentials[0]
	password := credentials[1]
	// ask for persistdb if user and password match
	authOk, err := persistdb.AuthenticateUser(username, password)
	if err != nil {
		return err
	}
	if !authOk {
		return fmt.Errorf("authentication failed")
	}
	// set username to configurations
	(*configurations)["username"] = username

	return nil
}

func buildProtocolHeader(protocol string) ([]byte, error) {
	parts := strings.Split(protocol, " ")
	if len(parts) != 2 || parts[0] != "AMQP" {
		return nil, fmt.Errorf("invalid protocol format: %s", protocol)
	}

	versionParts := strings.Split(parts[1], "-")
	if len(versionParts) != 3 {
		return nil, fmt.Errorf("invalid version format: %s", parts[1])
	}

	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %v", err)
	}
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %v", err)
	}
	revision, err := strconv.Atoi(versionParts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid revision version: %v", err)
	}

	header := []byte{
		'A', 'M', 'Q', 'P',
		0x00,
		byte(major),
		byte(minor),
		byte(revision),
	}

	return header, nil
}

func sendProtocolHeader(conn net.Conn, header []byte) error {
	_, err := conn.Write(header)
	return err
}

func readProtocolHeader(conn net.Conn) ([]byte, error) {
	header := make([]byte, 8)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func readFrame(conn net.Conn) ([]byte, error) {
	// all frames starts with a 7-octet header
	frameHeader := make([]byte, 7)
	_, err := io.ReadFull(conn, frameHeader)
	if err != nil {
		return nil, err
	}

	// fist octet is the type of frame
	// frameType := binary.BigEndian.Uint16(frameHeader[0:1])

	// 2nd and 3rd octets (short) are the channel number
	// channelNum := binary.BigEndian.Uint16(frameHeader[1:3])

	// 4th to 7th octets (long) are the size of the payload
	payloadSize := binary.BigEndian.Uint32(frameHeader[3:])

	// read the framePayload
	framePayload := make([]byte, payloadSize)
	_, err = io.ReadFull(conn, framePayload)
	if err != nil {
		return nil, err
	}

	// frame-end is a 1-octet after the payload
	frameEnd := make([]byte, 1)
	_, err = io.ReadFull(conn, frameEnd)
	if err != nil {
		return nil, err
	}

	// check if the frame-end is correct (0xCE)
	if frameEnd[0] != FRAME_END {
		// return nil, ErrInvalidFrameEnd
		return nil, fmt.Errorf("invalid frame end octet")
	}

	return append(frameHeader, framePayload...), nil
}

func sendFrame(conn net.Conn, frame []byte) error {
	log.Trace().Msgf("Sending frame: %x\n", frame)
	_, err := conn.Write(frame)
	return err
}
