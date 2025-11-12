package amqp

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"
)

// EncodeTable encodes a proper AMQP field table
func EncodeTable(table map[string]any) []byte {
	var buf bytes.Buffer
	for key, value := range table {
		// Field name
		EncodeShortStr(&buf, key)

		// Field value type and value
		switch v := value.(type) {
		case bool:
			buf.WriteByte('t')
			if v {
				buf.WriteByte(1)
			} else {
				buf.WriteByte(0)
			}

		case int8:
			buf.WriteByte('b')                          // Field value type 'b' (signed 8-bit int)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case uint8:
			buf.WriteByte('B')                          // Field value type 'B' (unsigned 8-bit int)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case int16:
			buf.WriteByte('U')                          // Field value type 'U' (signed 16-bit int)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case uint16:
			buf.WriteByte('u')                          // Field value type 'u' (unsigned 16-bit int)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case int32:
			buf.WriteByte('I')                          // Field value type 'I' (int32)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case int:
			buf.WriteByte('I')                          // Field value type 'I' (int32)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case uint32:
			buf.WriteByte('i')                          // Field value type 'i' (uint32)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case int64:
			buf.WriteByte('L')                          // Field value type 'L' (int64)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case uint64:
			buf.WriteByte('l')                          // Field value type 'l' (uint64)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case float32:
			buf.WriteByte('f')                          // Field value type 'f' (float32)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		case float64:
			buf.WriteByte('d')                          // Field value type 'd' (float64)
			_ = binary.Write(&buf, binary.BigEndian, v) // Error ignored as bytes.Buffer.Write never fails

		// realize how to encode amqp decimal types -- 1 byte scale + 4 byte value
		// for now, skip decimal encoding
		case decimal:
			buf.WriteByte('D') // Field value type 'D' (decimal)
			_ = binary.Write(&buf, binary.BigEndian, v.Scale)
			_ = binary.Write(&buf, binary.BigEndian, v.Value)

		case string:
			buf.WriteByte('S') // Field value type 'S' (long-string)
			EncodeLongStr(&buf, []byte(v))

		case map[string]any:
			// Recursively encode the nested map
			buf.WriteByte('F') // Field value type 'F' (field table)
			encodedTable := EncodeTable(v)
			EncodeLongStr(&buf, encodedTable)

		case Table:
			// Recursively encode the nested map
			buf.WriteByte('F') // Field value type 'F' (field table)
			encodedTable := EncodeTable(v)
			EncodeLongStr(&buf, encodedTable)

		case []any:
			// Array type 'A'
			buf.WriteByte('A')
			arrayBuf := EncodeArray(v)
			_ = binary.Write(&buf, binary.BigEndian, uint32(len(arrayBuf)))
			buf.Write(arrayBuf)

		case []string:
			// Array of strings - convert to []any and encode
			buf.WriteByte('A')
			arr := make([]any, len(v))
			for i, item := range v {
				arr[i] = item
			}
			arrayBuf := EncodeArray(arr)
			_ = binary.Write(&buf, binary.BigEndian, uint32(len(arrayBuf)))
			buf.Write(arrayBuf)

		case []map[string]any:
			// Array of tables - convert to []any and encode
			buf.WriteByte('A')
			arr := make([]any, len(v))
			for i, item := range v {
				arr[i] = item
			}
			arrayBuf := EncodeArray(arr)
			_ = binary.Write(&buf, binary.BigEndian, uint32(len(arrayBuf)))
			buf.Write(arrayBuf)

		case []Table:
			buf.WriteByte('A')
			arr := make([]any, len(v))
			for i, item := range v {
				arr[i] = item
			}
			arrayBuf := EncodeArray(arr)
			_ = binary.Write(&buf, binary.BigEndian, uint32(len(arrayBuf)))
			buf.Write(arrayBuf)

		default:
			buf.WriteByte('V') // Void/null type
		}
	}
	return buf.Bytes()
}

// EncodeArray encodes an AMQP array
func EncodeArray(arr []interface{}) []byte {
	var buf bytes.Buffer
	for _, value := range arr {
		switch v := value.(type) {
		case string:
			buf.WriteByte('S')
			EncodeLongStr(&buf, []byte(v))

		case int:
			buf.WriteByte('I')
			_ = binary.Write(&buf, binary.BigEndian, int32(v))

		case int32:
			buf.WriteByte('I')
			_ = binary.Write(&buf, binary.BigEndian, v)

		case uint32:
			buf.WriteByte('i') // lowercase 'i' for unsigned
			_ = binary.Write(&buf, binary.BigEndian, v)

		case map[string]any:
			buf.WriteByte('F') // Field table
			encodedTable := EncodeTable(v)
			_ = binary.Write(&buf, binary.BigEndian, uint32(len(encodedTable)))
			buf.Write(encodedTable)

		case bool:
			buf.WriteByte('t')
			if v {
				buf.WriteByte(1)
			} else {
				buf.WriteByte(0)
			}

		default:
			buf.WriteByte('V') // Void/null
		}
	}
	return buf.Bytes()
}

func EncodeLongStr(buf *bytes.Buffer, data []byte) {
	_ = binary.Write(buf, binary.BigEndian, uint32(len(data))) // Error ignored as bytes.Buffer.Write never fails
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
	_ = binary.Write(buf, binary.BigEndian, length) // Error ignored as bytes.Buffer.Write never fails
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
