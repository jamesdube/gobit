package main

import (
	"github.com/jamesdube/gobit/lib/event"
	"github.com/streadway/amqp"
	"log"
)

func main() {

	connection, err := amqp.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}
	defer connection.Close()

	topics := []string{"info"}

	consumer, err := event.NewConsumer("logs_topic",topics,"info",connection)
	if err != nil {
		panic(err)
	}
	consumer.Listen(func(b []byte) {
		log.Printf("Received a message: %s", b)
	})
}
