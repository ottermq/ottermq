package amqp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog/log"
)

// decodeTable decodes an AMQP field table from a byte slice
func DecodeTable(data []byte) (map[string]interface{}, error) {

	table := make(map[string]interface{})
	buf := bytes.NewReader(data)

	for buf.Len() > 0 {
		// Read field name
		fieldNameLength, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}

		fieldName := make([]byte, fieldNameLength)
		_, err = buf.Read(fieldName)
		if err != nil {
			return nil, err
		}

		// Read field value type
		fieldType, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}

		// Read field value based on the type
		switch fieldType {
		case 't':
			value, err := DecodeBoolean(buf)
			if err != nil {
				return nil, err
			}
			table[string(fieldName)] = value

		case 'b': // Signed 8-bit integer
			var intValue int8
			if err := binary.Read(buf, binary.BigEndian, &intValue); err != nil {
				return nil, err
			}
			table[string(fieldName)] = intValue

		case 'B': // Unsigned 8-bit integer
			var uintValue uint8
			if err := binary.Read(buf, binary.BigEndian, &uintValue); err != nil {
				return nil, err
			}
			table[string(fieldName)] = uintValue

		case 'U': // signed short integer (16-bit)
			var shortIntValue int16
			if err := binary.Read(buf, binary.BigEndian, &shortIntValue); err != nil {
				return nil, err
			}
			table[string(fieldName)] = shortIntValue

		case 'u': // Unsigned short integer (16-bit)
			var ushortIntValue uint16
			if err := binary.Read(buf, binary.BigEndian, &ushortIntValue); err != nil {
				return nil, err
			}
			table[string(fieldName)] = ushortIntValue

		case 'I': // Long-int (signed 32-bit)
			val, err := DecodeLongInt(buf)
			if err != nil {
				return nil, err
			}
			table[string(fieldName)] = val

		case 'i': // Long-uint (unsigned 32-bit)
			val, err := DecodeLongUInt(buf)
			if err != nil {
				return nil, err
			}
			table[string(fieldName)] = val

		case 'L': // Long long signed integer (64-bit)
			val, err1 := DecodeLongLongInt(buf)
			if err1 != nil {
				return nil, err
			}
			table[string(fieldName)] = val

		case 'l': // Long long unsigned integer (64-bit)
			val, err := DecodeLongLongUInt(buf)
			if err != nil {
				return nil, err
			}
			table[string(fieldName)] = val

		case 'f': // float (32-bit)
			var floatValue float32
			if err := binary.Read(buf, binary.BigEndian, &floatValue); err != nil {
				return nil, err
			}
			table[string(fieldName)] = floatValue

		case 'd': // double (64-bit)
			var doubleValue float64
			if err := binary.Read(buf, binary.BigEndian, &doubleValue); err != nil {
				return nil, err
			}
			table[string(fieldName)] = doubleValue

		case 'D': // Decimal type
			// 1 byte scale + 4 bytes value
			var scale uint8
			if err := binary.Read(buf, binary.BigEndian, &scale); err != nil {
				return nil, err
			}
			var decimalValue uint32
			if err := binary.Read(buf, binary.BigEndian, &decimalValue); err != nil {
				return nil, err
			}
			table[string(fieldName)] = struct {
				Scale uint8
				Value uint32
			}{Scale: scale, Value: decimalValue}

		case 's': // Short String
			strValue, err := DecodeShortStr(buf)
			if err != nil {
				return nil, err
			}

			table[string(fieldName)] = strValue

		case 'S': // Long String
			strValue, err := DecodeLongStr(buf)
			if err != nil {
				return nil, err
			}

			table[string(fieldName)] = strValue

		case 'A': // Array
			var arrayLength uint32
			if err := binary.Read(buf, binary.BigEndian, &arrayLength); err != nil {
				return nil, err
			}

			arrayData := make([]byte, arrayLength)
			_, err := buf.Read(arrayData)
			if err != nil {
				return nil, err
			}

			table[string(fieldName)] = arrayData

		case 'T': // Timestamp
			timestamp, err := DecodeTimestamp(buf)
			if err != nil {
				return nil, err
			}
			table[string(fieldName)] = timestamp

		case 'F': // Field Table
			var strLength uint32
			if err := binary.Read(buf, binary.BigEndian, &strLength); err != nil {
				return nil, err
			}

			strValue := make([]byte, strLength)
			_, err := buf.Read(strValue)
			if err != nil {
				return nil, err
			}

			value, err := DecodeTable(strValue)
			if err != nil {
				return nil, err
			}
			table[string(fieldName)] = value

		case 'V': // Void (null)
			// No payload, just store nil or skip
			table[string(fieldName)] = nil

		default:
			return nil, fmt.Errorf("unknown field type: %c", fieldType)
		}
	}
	return table, nil
}

// DecodeTimestamp reads and decodes a 64-bit POSIX timestamp from a bytes.Reader
func DecodeTimestamp(buf *bytes.Reader) (time.Time, error) {
	var timestamp int64
	err := binary.Read(buf, binary.BigEndian, &timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode timestamp: %v", err)
	}
	return time.Unix(timestamp, 0), nil
}

func DecodeLongStr(buf *bytes.Reader) (string, error) {
	var strLen uint32
	err := binary.Read(buf, binary.BigEndian, &strLen)
	if err != nil {
		return "", err
	}

	if strLen == 0 {
		return "", nil
	}

	strData := make([]byte, strLen)
	_, err = buf.Read(strData)
	if err != nil {
		return "", err
	}

	return string(strData), nil
}

func DecodeShortStr(buf *bytes.Reader) (string, error) {
	var strLen uint8
	err := binary.Read(buf, binary.BigEndian, &strLen)
	if err != nil {
		return "", err
	}

	strData := make([]byte, strLen)
	_, err = buf.Read(strData)
	if err != nil {
		return "", err
	}

	return string(strData), nil
}

func DecodeShortInt(buf *bytes.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(buf, binary.BigEndian, &value)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func DecodeLongInt(buf *bytes.Reader) (int32, error) {
	var longIntValue int32
	if err := binary.Read(buf, binary.BigEndian, &longIntValue); err != nil {
		return 0, err
	}
	return longIntValue, nil
}

func DecodeLongUInt(buf *bytes.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(buf, binary.BigEndian, &value)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func DecodeLongLongInt(buf *bytes.Reader) (int64, error) {
	var longLongInt int64
	if err := binary.Read(buf, binary.BigEndian, &longLongInt); err != nil {
		return 0, err
	}
	return longLongInt, nil
}

func DecodeLongLongUInt(buf *bytes.Reader) (uint64, error) {
	var value uint64
	err := binary.Read(buf, binary.BigEndian, &value)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func DecodeBoolean(buf *bytes.Reader) (bool, error) {
	var value uint8
	err := binary.Read(buf, binary.BigEndian, &value)
	if err != nil {
		return false, err
	}
	return value != 0, nil
}

// DecodeFlags decodes an octet into a map of flag names to boolean values.
func DecodeFlags(octet byte, flagNames []string, lsbFirst bool) map[string]bool {
	if len(flagNames) > 8 {
		log.Warn().Msg("More than 8 flag names provided; extra names will be ignored")
	}
	copiedFlagNames := make([]string, min(len(flagNames), 8))
	copy(copiedFlagNames, flagNames)
	flags := make(map[string]bool)
	for i := range copiedFlagNames {
		bit := i
		if !lsbFirst {
			bit = 7 - i
		}
		flags[copiedFlagNames[i]] = (octet & (1 << uint(bit))) != 0
	}
	return flags
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func DecodeSecurityPlain(buf *bytes.Reader) (string, error) {
	var strLen uint32
	err := binary.Read(buf, binary.BigEndian, &strLen)
	if err != nil {
		return "", err
	}

	if uint32(buf.Len()) < strLen {
		log.Debug().Int("buf_len", buf.Len()).Msg("Reached EOF")
		return "", io.EOF
	}

	// Read each byte and replace 0x00 with a space
	strData := make([]byte, strLen)
	for i := uint32(0); i < strLen; i++ {
		b, err := buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if b == 0x00 {
			strData[i] = ' '
		} else {
			strData[i] = b
		}
	}

	return string(strData), nil
}
