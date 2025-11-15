package amqp

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type XDeathEntry map[string]any

func encodeHeaders(buf *bytes.Buffer, table map[string]any) {
	// log.Debug().Interface("headers", table).Msg("Encoding headers in BasicProperties")
	encodedTable := EncodeTable(table)
	EncodeLongStr(buf, encodedTable)
	// log.Debug().Int("encoded_size", len(encodedTable)).Msg("Encoded table size")
	log.Debug().Hex("encoded_table", encodedTable).Msg("Encoded headers table")
}

// encodeValueToBuffer encodes a single AMQP field value into the provided buffer
// by selecting the appropriate type encoding based on the value's Go type.
func encodeValueToBuffer(value any, buf *bytes.Buffer) {
	switch v := value.(type) {
	case bool:
		buf.WriteByte('t')
		if v {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}

	case int8:
		buf.WriteByte('b') // Field value type 'b' (signed 8-bit int)
		_ = binary.Write(buf, binary.BigEndian, v)

	case uint8:
		buf.WriteByte('B') // Field value type 'B' (unsigned 8-bit int)
		_ = binary.Write(buf, binary.BigEndian, v)

	case int16:
		buf.WriteByte('U') // Field value type 'U' (signed 16-bit int)
		_ = binary.Write(buf, binary.BigEndian, v)

	case uint16:
		buf.WriteByte('u') // Field value type 'u' (unsigned 16-bit int)
		_ = binary.Write(buf, binary.BigEndian, v)

	case int32, int:
		buf.WriteByte('I') // Field value type 'I' (int32)
		_ = binary.Write(buf, binary.BigEndian, v)

	case uint32:
		buf.WriteByte('i') // Field value type 'i' (uint32)
		_ = binary.Write(buf, binary.BigEndian, v)

	case int64:
		buf.WriteByte('L') // Field value type 'L' (int64)
		_ = binary.Write(buf, binary.BigEndian, v)

	case uint64:
		buf.WriteByte('l') // Field value type 'l' (uint64)
		_ = binary.Write(buf, binary.BigEndian, v)

	case float32:
		buf.WriteByte('f') // Field value type 'f' (float32)
		_ = binary.Write(buf, binary.BigEndian, v)

	case float64:
		buf.WriteByte('d') // Field value type 'd' (float64)
		_ = binary.Write(buf, binary.BigEndian, v)

	// realize how to encode amqp decimal types -- 1 byte scale + 4 byte value
	// for now, skip decimal encoding
	case decimal:
		buf.WriteByte('D') // Field value type 'D' (decimal)
		_ = binary.Write(buf, binary.BigEndian, v.Scale)
		_ = binary.Write(buf, binary.BigEndian, v.Value)

	case string:
		buf.WriteByte('S') // Field value type 'S' (long-string)
		EncodeLongStr(buf, []byte(v))

	case map[string]any:
		log.Trace().Msg("Encoding map[string]any")
		buf.WriteByte('F') // Field value type 'F' (field table)
		data := EncodeTable(v)
		EncodeLongStr(buf, data)

	case []map[string]any:
		log.Trace().Msg("Encoding array of map[string]any")
		buf.WriteByte('A')
		arr := make([]any, len(v))
		for i, item := range v {
			arr[i] = item
		}
		data := EncodeArray(arr)
		EncodeLongStr(buf, data)

	case []string:
		// Array of strings - convert to []any and encode
		buf.WriteByte('A')
		arr := make([]any, len(v))
		for i, item := range v {
			arr[i] = item
		}
		data := EncodeArray(arr)
		EncodeLongStr(buf, data)

	case time.Time:
		log.Trace().Msg("Encoding time.Time value")
		buf.WriteByte('T') // Timestamp
		timestamp := v.Unix()
		binary.Write(buf, binary.BigEndian, uint64(timestamp))

	default:
		log.Warn().Interface("value", v).Msg("Unsupported AMQP field value type, encoding as null")
		buf.WriteByte('V') // Void/null type
	}
}

// EncodeTable encodes a proper AMQP field table
func EncodeTable(table map[string]any) []byte {
	var buf bytes.Buffer
	for key, value := range table {
		// Field name
		log.Trace().Str("key", key).Interface("value", value).Msg("Encoding table entry")
		if key == "x-death" {
			xDeathBuf := bytes.Buffer{}
			// Encode as array of field tables
			xDeathArray, ok := value.([]map[string]any)
			if !ok {
				log.Warn().Interface("x-death", value).Msg("x-death header is not in expected format, encoding as null")
				EncodeShortStr(&buf, key)
				buf.WriteByte('V') // Void/null type
				continue
			}

			var arrayContent bytes.Buffer
			for _, entry := range xDeathArray {
				log.Trace().Msg("Encoding x-death entry as field table")
				var entryBuf bytes.Buffer
				entryBuf.WriteByte('F') // Field table type
				entryData := EncodeTable(entry)
				EncodeLongStr(&entryBuf, entryData)
				arrayContent.Write(entryBuf.Bytes())
			}
			// Write length of array content
			_ = binary.Write(&xDeathBuf, binary.BigEndian, uint32(arrayContent.Len()))
			xDeathBuf.Write(arrayContent.Bytes())

			// Now write to main buffer
			EncodeShortStr(&buf, key)
			buf.WriteByte('A') // Array type
			buf.Write(xDeathBuf.Bytes())
			log.Trace().Msg("Encoding x-death header")
			log.Trace().Hex("x-death-encoded", xDeathBuf.Bytes()).Int("size", len(xDeathBuf.Bytes())).Msg("Encoded x-death header")

		} else {
			EncodeShortStr(&buf, key)

			// Field value type and value
			encodeValueToBuffer(value, &buf)
		}
	}
	return buf.Bytes()
}

func EncodeArray(arr any) []byte {
	var buf bytes.Buffer
	for _, value := range arr.([]any) {
		encodeValueToBuffer(value, &buf)
	}
	return buf.Bytes()
}

func EncodeLongStr(buf *bytes.Buffer, data []byte) {
	_ = binary.Write(buf, binary.BigEndian, uint32(len(data)))
	buf.Write(data)
}

func EncodeShortStr(buf *bytes.Buffer, data string) {
	_ = buf.WriteByte(byte(len(data)))
	buf.WriteString(data)
}

func EncodeOctet(buf *bytes.Buffer, value uint8) error {
	return buf.WriteByte(value)
}

func EncodeTimestamp(buf *bytes.Buffer, value time.Time) error {
	timestamp := value.Unix()
	return binary.Write(buf, binary.BigEndian, timestamp)
}

func EncodeSecurityPlain(buf *bytes.Buffer, securityStr string) []byte {
	// Concatenate username, null byte, and password
	// securityStr := username + "\x00" + password
	// Replace spaces with null bytes
	encodedStr := strings.ReplaceAll(securityStr, " ", "\x00")
	// Encode length as a uint32 and append the encoded string
	length := uint32(len(encodedStr))
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.WriteString(encodedStr)
	return buf.Bytes()
}

// EncodeFlags encodes a map of boolean flags into a single byte
func EncodeFlags(flags map[string]bool, flagNames []string, lsbFirst bool) byte {
	var octet byte = 0
	for i, name := range flagNames {
		if flags[name] {
			if lsbFirst {
				octet |= 1 << i
			} else {
				octet |= 1 << (7 - i)
			}
		}
	}
	return octet
}
