package event

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
)

func getExchangeName() string {
	return "logs_topic"
}

func declareQueue(ch *amqp.Channel, name string, durable bool) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,    // name
		durable, // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
}

func declareExchange(ch *amqp.Channel, exchange string) error {
	return ch.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
}

// NewConnectionManagerFromURL creates a new connection manager from a URL
func NewConnectionManagerFromURL(url string) (*ConnectionManager, error) {
	return NewConnectionManager(url, DefaultConnectionConfig(), slog.Default())
}

// NewConnectionManagerFromURLWithConfig creates a new connection manager from a URL with custom config
func NewConnectionManagerFromURLWithConfig(url string, config ConnectionConfig, logger *slog.Logger) (*ConnectionManager, error) {
	return NewConnectionManager(url, config, logger)
}
