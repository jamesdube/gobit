package event

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
)

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

// declareQueueWithDLQ declares a queue wired to a dead-letter exchange.
// Messages that exceed retry limits are routed to {dlxExchange} with routing key {queue}.dlq.
func declareQueueWithDLQ(ch *amqp.Channel, name string, durable bool, dlxExchange string) (amqp.Queue, error) {
	args := amqp.Table{
		"x-dead-letter-exchange":    dlxExchange,
		"x-dead-letter-routing-key": name + ".dlq",
	}
	return ch.QueueDeclare(
		name,    // name
		durable, // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		args,    // arguments
	)
}

// declareDLQ declares the dead-letter exchange and DLQ queue, then binds them.
// Convention: exchange = {originalExchange}.dlx, queue = {originalQueue}.dlq.
func declareDLQ(ch *amqp.Channel, queueName string, exchangeName string) error {
	dlxExchange := exchangeName + ".dlx"
	dlqQueue := queueName + ".dlq"

	// Declare the DLX as a direct exchange (messages arrive with a specific routing key)
	err := ch.ExchangeDeclare(
		dlxExchange, // name
		"direct",    // type
		true,        // durable
		false,       // auto-deleted
		false,       // internal
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		return err
	}

	// Declare the DLQ queue
	_, err = ch.QueueDeclare(
		dlqQueue, // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return err
	}

	// Bind DLQ to DLX with routing key = {queue}.dlq
	return ch.QueueBind(dlqQueue, dlqQueue, dlxExchange, false, nil)
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
