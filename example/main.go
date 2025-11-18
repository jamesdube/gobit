package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesdube/gobit/lib/event"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.Handler{Level: slog.LevelDebug}))

	// Create connection manager with custom configuration
	config := event.DefaultConnectionConfig()
	config.MaxRetries = 10
	config.ReconnectDelay = 2 * time.Second

	connManager, err := event.NewConnectionManagerFromURLWithConfig(
		"amqp://guest:guest@localhost:5672",
		config,
		logger,
	)
	if err != nil {
		logger.Error("Failed to create connection manager", "error", err)
		os.Exit(1)
	}
	defer connManager.Close()

	// Create emitter
	emitter, err := event.NewEventEmitterWithConfig(connManager, config, logger)
	if err != nil {
		logger.Error("Failed to create emitter", "error", err)
		os.Exit(1)
	}
	defer emitter.Close()

	// Create consumer
	topics := []string{"notifications.*", "events.*"}
	consumer, err := event.NewConsumerWithConfig(
		connManager,
		"logs_topic",
		topics,
		"example_queue",
		true, // durable
		config,
		logger,
	)
	if err != nil {
		logger.Error("Failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// Start consumer in a goroutine
	go func() {
		err := consumer.Listen(func(data []byte) error {
			logger.Info("Received message", "data", string(data))
			return nil
		})
		if err != nil {
			logger.Error("Consumer stopped", "error", err)
		}
	}()

	// Send some messages
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 10; i++ {
			select {
			case <-ticker.C:
				message := fmt.Sprintf(`{"type": "notification", "message": "Hello %d", "timestamp": "%s"}`, i, time.Now().Format(time.RFC3339))
				err := emitter.Publish("logs_topic", "notifications.example", message)
				if err != nil {
					logger.Error("Failed to publish message", "error", err)
				} else {
					logger.Info("Published message", "message", message)
				}
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
}