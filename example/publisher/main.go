package main

import (
	"fmt"
	"github.com/jamesdube/gobit/lib/event"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	//	"time"
)

func main() {
	logger := slog.Default()
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}

	emitter, err := event.NewEventEmitter(conn)
	if err != nil {
		panic(err)
	}

	for i := 1; i < 100; i++ {
		err := emitter.Publish("sms", "sms.econet", fmt.Sprintf("message number [%d]", i))
		if err != nil {
			logger.Info("error", err.Error())
			return
		}
		logger.Info("published message", "id", i)
		//		time.Sleep(100 * time.Millisecond)
	}
}
