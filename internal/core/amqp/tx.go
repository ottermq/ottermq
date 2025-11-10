package amqp

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

type TxSelect struct{}

type TxCommit struct{}

type TxRollback struct{}

// createTxAckFrame creates a TX method acknowledgment frame
// to be used for TX_SELECT_OK, TX_COMMIT_OK, and TX_ROLLBACK_OK
func createTxAckFrame(channel uint16, methodID uint16) []byte {
	frame := ResponseMethodMessage{
		Channel:  channel,
		ClassID:  uint16(TX),
		MethodID: methodID,
		Content:  ContentList{},
	}.FormatMethodFrame()
	return frame
}

func parseTxMethod(methodID uint16, payload []byte) (any, error) {
	switch methodID {
	case uint16(TX_SELECT):
		log.Debug().Msg("Received TX_SELECT frame \n")
		return parseTxSelectFrame(payload)

	case uint16(TX_SELECT_OK):
		log.Debug().Msg("Received TX_SELECT_OK frame \n")
		log.Warn().Msg("Server should not receive TX_SELECT_OK frames from clients")
		return nil, fmt.Errorf("server should not receive TX_SELECT_OK frames from clients")

	case uint16(TX_COMMIT):
		log.Debug().Msg("Received TX_COMMIT frame \n")
		return parseTxCommitFrame(payload)

	case uint16(TX_COMMIT_OK):
		log.Debug().Msg("Received TX_COMMIT_OK frame \n")
		log.Warn().Msg("Server should not receive TX_COMMIT_OK frames from clients")
		return nil, fmt.Errorf("server should not receive TX_COMMIT_OK frames from clients")

	case uint16(TX_ROLLBACK):
		log.Debug().Msg("Received TX_ROLLBACK frame \n")
		return parseTxRollbackFrame(payload)

	case uint16(TX_ROLLBACK_OK):
		log.Debug().Msg("Received TX_ROLLBACK_OK frame \n")
		log.Warn().Msg("Server should not receive TX_ROLLBACK_OK frames from clients")
		return nil, fmt.Errorf("server should not receive TX_ROLLBACK_OK frames from clients")

	default:
		return nil, fmt.Errorf("unknown method ID: %d", methodID)
	}
}

func parseTxSelectFrame(_ []byte) (any, error) {
	content := &TxSelect{}
	request := &RequestMethodMessage{
		Content: content,
	}
	return request, nil
}

func parseTxCommitFrame(_ []byte) (any, error) {
	content := &TxCommit{}
	request := &RequestMethodMessage{
		Content: content,
	}
	return request, nil
}

func parseTxRollbackFrame(_ []byte) (any, error) {
	content := &TxRollback{}
	request := &RequestMethodMessage{
		Content: content,
	}
	return request, nil
}
