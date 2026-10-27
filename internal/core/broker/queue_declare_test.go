package broker

import (
	"net"
	"testing"

	"github.com/ottermq/ottermq/internal/core/amqp"
	"github.com/ottermq/ottermq/internal/core/broker/vhost"
	"github.com/ottermq/ottermq/pkg/persistence/implementations/dummy"
)

// declareOkRecorder records the counts the broker puts in queue.declare-ok.
type declareOkRecorder struct {
	amqp.DefaultFramer
	messageCount  uint32
	consumerCount uint32
	calls         int
}

func (r *declareOkRecorder) CreateQueueDeclareOkFrame(channel uint16, queueName string, messageCount, consumerCount uint32) ([]byte, error) {
	r.messageCount = messageCount
	r.consumerCount = consumerCount
	r.calls++
	return r.DefaultFramer.CreateQueueDeclareOkFrame(channel, queueName, messageCount, consumerCount)
}

// Regression test for #335: queue.declare-ok must report the queue's
// consumer count instead of always 0.
func TestQueueDeclareHandler_ReportsConsumerCount(t *testing.T) {
	recorder := &declareOkRecorder{}
	b := &Broker{
		framer:   recorder,
		connToID: make(map[net.Conn]vhost.ConnectionID),
	}
	vh := vhost.NewVhost("/", vhost.VHostOptions{
		QueueBufferSize: 100,
		Persistence:     &dummy.DummyPersistence{},
	})
	vh.SetFramer(b.framer)

	conn := &mockConn{}
	connID := newTestConsumerConnID()
	b.connToID[conn] = connID

	const queueName = "declare-ok.consumers"
	declare := func(passive bool) uint32 {
		t.Helper()
		before := recorder.calls
		req := &amqp.RequestMethodMessage{
			Channel:  1,
			ClassID:  uint16(amqp.QUEUE),
			MethodID: uint16(amqp.QUEUE_DECLARE),
			Content: &amqp.QueueDeclareMessage{
				QueueName: queueName,
				Passive:   passive,
			},
		}
		if _, err := b.queueDeclareHandler(req, vh, conn); err != nil {
			t.Fatalf("queueDeclareHandler(passive=%v) failed: %v", passive, err)
		}
		if recorder.calls != before+1 {
			t.Fatalf("queueDeclareHandler(passive=%v) did not send queue.declare-ok", passive)
		}
		return recorder.consumerCount
	}

	if got := declare(false); got != 0 {
		t.Errorf("new queue: consumer count = %d, want 0", got)
	}

	for i, tag := range []string{"ctag-1", "ctag-2"} {
		c := vhost.NewConsumer(connID, uint16(i+1), queueName, tag, &vhost.ConsumerProperties{})
		if _, err := vh.RegisterConsumer(c); err != nil {
			t.Fatalf("RegisterConsumer(%s) failed: %v", tag, err)
		}
	}

	if got := declare(false); got != 2 {
		t.Errorf("redeclare with 2 consumers: consumer count = %d, want 2", got)
	}
	if got := declare(true); got != 2 {
		t.Errorf("passive declare with 2 consumers: consumer count = %d, want 2", got)
	}

	if err := vh.CancelConsumer(1, "ctag-1"); err != nil {
		t.Fatalf("CancelConsumer failed: %v", err)
	}
	if got := declare(true); got != 1 {
		t.Errorf("passive declare after one cancel: consumer count = %d, want 1", got)
	}
}
