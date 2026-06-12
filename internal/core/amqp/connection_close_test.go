package amqp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestParseConnectionCloseFrame_ShortReplyText verifies that a Connection.Close
// frame declaring a reply-text length longer than the remaining payload returns
// an error instead of panicking with slice bounds out of range.
func TestParseConnectionCloseFrame_ShortReplyText(t *testing.T) {
	var payload bytes.Buffer
	_ = binary.Write(&payload, binary.BigEndian, uint16(200)) // replyCode
	payload.WriteByte(200)                                    // replyTextLen, but no reply text follows
	for payload.Len() < 12 {                                  // pad past the len(payload) < 12 guard
		payload.WriteByte(0)
	}

	_, err := parseConnectionCloseFrame(payload.Bytes())
	if err == nil {
		t.Fatal("Expected error for reply text length exceeding payload, got nil")
	}
}

// TestParseConnectionCloseFrame_Valid confirms a well-formed frame still parses.
func TestParseConnectionCloseFrame_Valid(t *testing.T) {
	var payload bytes.Buffer
	_ = binary.Write(&payload, binary.BigEndian, uint16(320)) // replyCode
	replyText := "CONNECTION_FORCED"
	payload.WriteByte(byte(len(replyText)))
	payload.WriteString(replyText)
	_ = binary.Write(&payload, binary.BigEndian, uint16(CONNECTION)) // classID
	_ = binary.Write(&payload, binary.BigEndian, uint16(0))          // methodID

	request, err := parseConnectionCloseFrame(payload.Bytes())
	if err != nil {
		t.Fatalf("parseConnectionCloseFrame failed: %v", err)
	}
	msg, ok := request.Content.(*ConnectionCloseMessage)
	if !ok {
		t.Fatalf("Expected *ConnectionCloseMessage, got %T", request.Content)
	}
	if msg.ReplyText != replyText {
		t.Errorf("Expected reply text %q, got %q", replyText, msg.ReplyText)
	}
}
