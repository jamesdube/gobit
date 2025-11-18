package main

import (
	"github.com/jamesdube/gobit/lib/event"
	"log/slog"
)

var logger = slog.Default()

func main() {

	connManager, err := event.NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}
	defer connManager.Close()

	topics := []string{"sms.econet", "sms.netone"}

	consumer, err := event.NewConsumer(connManager, "sms", topics, "sms", true)
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
