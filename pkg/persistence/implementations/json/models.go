package json

import "github.com/ottermq/ottermq/pkg/persistence"

type JsonMessageData struct {
	ID         string                        `json:"id"`
	Body       []byte                        `json:"body"`
	Properties persistence.MessageProperties `json:"properties"`
}

type JsonQueueData struct {
	Name       string                      `json:"name"`
	Properties persistence.QueueProperties `json:"properties"`
	Messages   []JsonMessageData           `json:"messages"`
}

type JsonExchangeData struct {
	Name       string                         `json:"name"`
	Type       string                         `json:"type"`
	Properties persistence.ExchangeProperties `json:"properties"`
	Bindings   []persistence.BindingData      `json:"bindings"`
}

type JsonVHostData struct {
	Name      string             `json:"name"`
	Exchanges []JsonExchangeData `json:"exchanges"`
	Queues    []JsonQueueData    `json:"queues"`
}

type JsonBrokerData struct {
	VHosts  []JsonVHostData `json:"vhosts"`
	Version int             `json:"version"`
}
