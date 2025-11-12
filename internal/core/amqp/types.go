package amqp

type decimal struct {
	Scale uint8
	Value uint32
}

type Table map[string]any

type Array []any
