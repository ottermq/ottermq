package errors

import "fmt"

type ConnectionError struct {
	code     uint16
	text     string
	classID  uint16
	methodID uint16
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("AMQP Connection Error %d: %s", e.code, e.text)
}

func (e *ConnectionError) ReplyText() string {
	return e.text
}

func (e *ConnectionError) ReplyCode() uint16 {
	return e.code
}

func (e *ConnectionError) ClassID() uint16 {
	return e.classID
}

func (e *ConnectionError) MethodID() uint16 {
	return e.methodID
}

func NewConnectionError(text string, code, classID, methodID uint16) AMQPError {
	return &ConnectionError{
		text:     text,
		code:     code,
		classID:  classID,
		methodID: methodID,
	}
}
