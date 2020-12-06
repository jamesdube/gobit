package event

import (
	"github.com/streadway/amqp"
)

func getExchangeName() string {
	return "logs_topic"
}

func declareQueue(ch *amqp.Channel,q string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		q,    // name
		false, // durable
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
