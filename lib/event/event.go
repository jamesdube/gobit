package event

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

func getExchangeName() string {
	return "logs_topic"
}

func declareQueue(ch *amqp.Channel,name string, durable bool) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,    // name
		durable, // durable
		false, // delete when unused
		false,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

func declareExchange(ch *amqp.Channel,exchange string) error {
	return ch.ExchangeDeclare(
		exchange, // name
		"topic",           // type
		true,              // durable
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
}
