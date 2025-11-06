package amqp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestParseQueueDeleteFrame_FlagDecoding verifies ifUnused, ifEmpty, noWait bits
func TestParseQueueDeleteFrame_FlagDecoding(t *testing.T) {
	var payload bytes.Buffer
	// reserved1 (short) = 0
	_ = binary.Write(&payload, binary.BigEndian, uint16(0))
	// queue name
	EncodeShortStr(&payload, "qdel.flags")
	// flags octet: bit0=ifUnused, bit1=ifEmpty, bit2=noWait
	// set ifUnused=true, ifEmpty=true, noWait=false => 0b0000011 = 0x03
	payload.WriteByte(0x03)

	req, err := parseQueueDeleteFrame(payload.Bytes())
	if err != nil {
		t.Fatalf("parseQueueDeleteFrame failed: %v", err)
	}
	msg, ok := req.Content.(*QueueDeleteMessage)
	if !ok {
		t.Fatalf("expected *QueueDeleteMessage, got %T", req.Content)
	}
	if msg.QueueName != "qdel.flags" {
		t.Errorf("unexpected queue name: %s", msg.QueueName)
	}
	if !msg.IfUnused {
		t.Errorf("expected IfUnused=true")
	}
	if !msg.IfEmpty {
		t.Errorf("expected IfEmpty=true")
	}
	if msg.NoWait {
		t.Errorf("expected NoWait=false")
	}
}

// TestParseQueueDeleteFrame_NoFlags verifies decoding when no flags are set
func TestParseQueueDeleteFrame_NoFlags(t *testing.T) {
	var payload bytes.Buffer
	_ = binary.Write(&payload, binary.BigEndian, uint16(0))
	EncodeShortStr(&payload, "qdel.noflags")
	payload.WriteByte(0x00)

	req, err := parseQueueDeleteFrame(payload.Bytes())
	if err != nil {
		t.Fatalf("parseQueueDeleteFrame failed: %v", err)
	}
	msg := req.Content.(*QueueDeleteMessage)
	if msg.IfUnused || msg.IfEmpty || msg.NoWait {
		t.Errorf("expected all flags false, got %+v", msg)
	}
}
