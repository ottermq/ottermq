package amqp

import "fmt"

const (
	INT_OCTET     = "octet"
	INT_SHORT     = "short"
	INT_LONG      = "long"
	INT_LONG_LONG = "long-long"
	BIT           = "bit"
	STRING_SHORT  = "short-str"
	STRING_LONG   = "long-str"
	TIMESTAMP     = "timestamp"
	TABLE         = "table"
)

type TypeMethod uint16

type QueueMethod int

const (
	QUEUE_DECLARE    QueueMethod = 10
	QUEUE_DECLARE_OK QueueMethod = 11
	QUEUE_BIND       QueueMethod = 20
	QUEUE_BIND_OK    QueueMethod = 21
	QUEUE_UNBIND     QueueMethod = 50
	QUEUE_UNBIND_OK  QueueMethod = 51
	QUEUE_PURGE      QueueMethod = 30
	QUEUE_PURGE_OK   QueueMethod = 31
	QUEUE_DELETE     QueueMethod = 40
	QUEUE_DELETE_OK  QueueMethod = 41
)

type FrameType uint8

const (
	TYPE_METHOD    FrameType = 1
	TYPE_HEADER    FrameType = 2
	TYPE_BODY      FrameType = 3
	TYPE_HEARTBEAT FrameType = 8
)

type ExchangeMethod int

const (
	EXCHANGE_DECLARE    ExchangeMethod = 10
	EXCHANGE_DECLARE_OK ExchangeMethod = 11
	EXCHANGE_DELETE     ExchangeMethod = 20
	EXCHANGE_DELETE_OK  ExchangeMethod = 21
)

const (
	CONNECTION_START     TypeMethod = 10
	CONNECTION_START_OK  TypeMethod = 11
	CONNECTION_SECURE    TypeMethod = 20
	CONNECTION_SECURE_OK TypeMethod = 21
	CONNECTION_TUNE      TypeMethod = 30
	CONNECTION_TUNE_OK   TypeMethod = 31
	CONNECTION_OPEN      TypeMethod = 40
	CONNECTION_OPEN_OK   TypeMethod = 41
	CONNECTION_CLOSE     TypeMethod = 50
	CONNECTION_CLOSE_OK  TypeMethod = 51
)

type TypeClass int

// Class constants
const (
	CONNECTION TypeClass = 10
	CHANNEL    TypeClass = 20
	EXCHANGE   TypeClass = 40
	QUEUE      TypeClass = 50
	BASIC      TypeClass = 60
	TX         TypeClass = 90
)

type ChannelMethod int

const (
	CHANNEL_OPEN     ChannelMethod = 10
	CHANNEL_OPEN_OK  ChannelMethod = 11
	CHANNEL_FLOW     ChannelMethod = 20
	CHANNEL_FLOW_OK  ChannelMethod = 21
	CHANNEL_CLOSE    ChannelMethod = 40
	CHANNEL_CLOSE_OK ChannelMethod = 41
)

type BasicMethod int

const (
	BASIC_QOS           BasicMethod = 10
	BASIC_QOS_OK        BasicMethod = 11
	BASIC_CONSUME       BasicMethod = 20
	BASIC_CONSUME_OK    BasicMethod = 21
	BASIC_CANCEL        BasicMethod = 30
	BASIC_CANCEL_OK     BasicMethod = 31
	BASIC_PUBLISH       BasicMethod = 40
	BASIC_RETURN        BasicMethod = 50
	BASIC_DELIVER       BasicMethod = 60
	BASIC_GET           BasicMethod = 70
	BASIC_GET_OK        BasicMethod = 71
	BASIC_GET_EMPTY     BasicMethod = 72
	BASIC_ACK           BasicMethod = 80
	BASIC_REJECT        BasicMethod = 90
	BASIC_RECOVER_ASYNC BasicMethod = 100
	BASIC_RECOVER       BasicMethod = 110
	BASIC_RECOVER_OK    BasicMethod = 111
	BASIC_NACK          BasicMethod = 120
)

type TxMethod int

const (
	SELECT      TxMethod = 10
	SELECT_OK   TxMethod = 11
	COMMIT      TxMethod = 20
	COMMIT_OK   TxMethod = 21
	ROLLBACK    TxMethod = 30
	ROLLBACK_OK TxMethod = 31
)

// AMQP Reply Codes as defined in AMQP 0-9-1 specification
type ReplyCode uint16

const (
	REPLY_SUCCESS       ReplyCode = 200 // reply-success
	CONTENT_TOO_LARGE   ReplyCode = 311 // content-too-large
	NO_ROUTE            ReplyCode = 312 // no-route
	NO_CONSUMERS        ReplyCode = 313 // no-consumers
	CONNECTION_FORCED   ReplyCode = 320 // connection-forced
	INVALID_PATH        ReplyCode = 402 // invalid-path
	ACCESS_REFUSED      ReplyCode = 403 // access-refused
	NOT_FOUND           ReplyCode = 404 // not-found
	RESOURCE_LOCKED     ReplyCode = 405 // resource-locked
	PRECONDITION_FAILED ReplyCode = 406 // precondition-failed
	FRAME_ERROR         ReplyCode = 501 // frame-error
	SYNTAX_ERROR        ReplyCode = 502 // syntax-error
	COMMAND_INVALID     ReplyCode = 503 // command-invalid
	CHANNEL_ERROR       ReplyCode = 504 // channel-error
	UNEXPECTED_FRAME    ReplyCode = 505 // unexpected-frame
	RESOURCE_ERROR      ReplyCode = 506 // resource-error
	NOT_ALLOWED         ReplyCode = 530 // not-allowed
	NOT_IMPLEMENTED     ReplyCode = 540 // not-implemented
	INTERNAL_ERROR      ReplyCode = 541 // internal-error
)

// ReplyText returns the default reply text for a given reply code
var ReplyText = map[ReplyCode]string{
	REPLY_SUCCESS:       "REPLY_SUCCESS",
	CONTENT_TOO_LARGE:   "CONTENT_TOO_LARGE",
	NO_ROUTE:            "NO_ROUTE",
	NO_CONSUMERS:        "NO_CONSUMERS",
	CONNECTION_FORCED:   "CONNECTION_FORCED",
	INVALID_PATH:        "INVALID_PATH",
	ACCESS_REFUSED:      "ACCESS_REFUSED",
	NOT_FOUND:           "NOT_FOUND",
	RESOURCE_LOCKED:     "RESOURCE_LOCKED",
	PRECONDITION_FAILED: "PRECONDITION_FAILED",
	FRAME_ERROR:         "FRAME_ERROR",
	SYNTAX_ERROR:        "SYNTAX_ERROR",
	COMMAND_INVALID:     "COMMAND_INVALID",
	CHANNEL_ERROR:       "CHANNEL_ERROR",
	UNEXPECTED_FRAME:    "UNEXPECTED_FRAME",
	RESOURCE_ERROR:      "RESOURCE_ERROR",
	NOT_ALLOWED:         "NOT_ALLOWED",
	NOT_IMPLEMENTED:     "NOT_IMPLEMENTED",
	INTERNAL_ERROR:      "INTERNAL_ERROR",
}

func (rc ReplyCode) String() string {
	if text, exists := ReplyText[rc]; exists {
		return text
	}
	return "UNKNOWN_REPLY_CODE"
}

func (rc ReplyCode) Format(reason string) string {
	return fmt.Sprintf("%s - %s", rc.String(), reason)
}

type DeliveryMode uint8

const (
	DEFAULT        DeliveryMode = 0 // It means the same as NON_PERSISTENT
	NON_PERSISTENT DeliveryMode = 1
	PERSISTENT     DeliveryMode = 2
)

func (dm DeliveryMode) String() string {
	return []string{"default", "non-persistent", "persistent"}[dm]
}

func (dm DeliveryMode) Validate() error {
	if dm != 0 && dm != 1 && dm != 2 {
		return fmt.Errorf("invalid delivery mode: %d (must be 0, 1, or 2)", dm)
	}
	return nil
}

func (dm DeliveryMode) Normalize() DeliveryMode {
	if dm == 0 {
		return NON_PERSISTENT
	}
	return dm
}

type ContentType string

// Common AMQP content types
const (
	TEXT_PLAIN       ContentType = "text/plain"
	TEXT_HTML        ContentType = "text/html"
	APPLICATION_JSON ContentType = "application/json"
	APPLICATION_XML  ContentType = "application/xml"
	IMAGE_PNG        ContentType = "image/png"
	IMAGE_JPEG       ContentType = "image/jpeg"
	MULTIPART_FORM   ContentType = "multipart/form-data"
)
