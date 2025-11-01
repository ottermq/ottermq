package errors

import "fmt"

type ChannelError struct {
	code     uint16
	text     string
	classID  uint16
	methodID uint16
}

func (e *ChannelError) Error() string {
	return fmt.Sprintf("AMQP Channel Error %d: %s", e.code, e.text)
}

func (e *ChannelError) ReplyText() string {
	return e.text
}

func (e *ChannelError) ReplyCode() uint16 {
	return e.code
}

func (e *ChannelError) ClassID() uint16 {
	return e.classID
}

func (e *ChannelError) MethodID() uint16 {
	return e.methodID
}

func NewChannelError(text string, code, classID, methodID uint16) AMQPError {
	return &ChannelError{
		text:     text,
		code:     code,
		classID:  classID,
		methodID: methodID,
	}
}
