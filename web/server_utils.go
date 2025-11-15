package web

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/rabbitmq/amqp091-go"
)

func GetBrokerClient(c *Config) (*amqp091.Connection, error) {
	username := c.Username
	password := c.Password
	host := c.BrokerHost
	port := c.BrokerPort

	connectionString := fmt.Sprintf("amqp://%s:%s@%s:%s/", username, password, host, port)
	brokerAddr := fmt.Sprintf("%s:%s", host, port)
	log.Debug().Str("addr", brokerAddr).Msg("Connecting to broker")

	conn, err := amqp091.Dial(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to broker: %w", err)
	}

	return conn, nil
}
