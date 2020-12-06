package main

import (
	"fmt"
	"github.com/jamesdube/gobit/lib/event"
	"github.com/streadway/amqp"

)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}

	emitter, err := event.NewEventEmitter(conn)
	if err != nil {
		panic(err)
	}

	for i := 1; i < 10; i++ {
		emitter.Publish("logs_topic","info",fmt.Sprintf("message number [%d]",i))
	}
}
