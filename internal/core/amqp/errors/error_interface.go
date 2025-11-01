package errors

type AMQPError interface {
	error
	ReplyText() string
	ReplyCode() uint16
	ClassID() uint16
	MethodID() uint16
}
