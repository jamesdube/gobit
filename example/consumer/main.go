package main

import (
	"github.com/jamesdube/gobit/lib/event"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
)

var logger = slog.Default()

func main() {

	connection, err := amqp.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}
	defer connection.Close()

	topics := []string{"sms.econet", "sms.netone"}

	consumer, err := event.NewConsumer(connection, "sms", topics, "sms", true)
	consumer.SetPrefetchCount(7)
	if err != nil {
		panic(err)
	}

	err = consumer.Listen(onMessageReceived)
	if err != nil {
		logger.Error("consumer error", "error", err.Error())
		return
	}
}

func onMessageReceived(b []byte) error {
	logger.Info("received message", "message", string(b))
	return nil
}
