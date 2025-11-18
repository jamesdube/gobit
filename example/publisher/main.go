package main

import (
	"fmt"
	"github.com/jamesdube/gobit/lib/event"
	"log/slog"
	//	"time"
)

func main() {
	logger := slog.Default()

	connManager, err := event.NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}
	defer connManager.Close()

	emitter, err := event.NewEventEmitter(connManager)
	if err != nil {
		panic(err)
	}

	for i := 1; i < 100; i++ {
		err := emitter.Publish("sms", "sms.econet", fmt.Sprintf("message number [%d]", i))
		if err != nil {
			logger.Info("error", "err", err.Error())
			return
		}
		logger.Info("published message", "id", i)
		//		time.Sleep(100 * time.Millisecond)
	}
}
