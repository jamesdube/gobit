package event

import (
	"log/slog"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestConnectionManagerCreateSuccess(t *testing.T) {
	connManager, err := NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Skipf("Could not establish a connection to AMQP server: %v", err)
	}
	defer connManager.Close()

	if !connManager.IsConnected() {
		t.Error("Connection manager should be connected")
	}
}

func TestEmitterCreateSuccess(t *testing.T) {
	connManager, err := NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Skipf("Could not establish a connection to AMQP server: %v", err)
	}
	defer connManager.Close()

	_, err = NewEventEmitter(connManager)
	if err != nil {
		t.Errorf("Error creating Event Emitter: %v", err)
	}
}

func TestEmitterPushSuccess(t *testing.T) {
	connManager, err := NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Skipf("Could not establish a connection to AMQP server: %v", err)
	}
	defer connManager.Close()

	emitter, err := NewEventEmitter(connManager)
	if err != nil {
		t.Errorf("Error creating Event Emitter: %v", err)
	}

	err = emitter.Publish("logs_topic", "test.log", "Hello World!")
	if err != nil {
		t.Errorf("Could not push to queue successfully: %v", err)
	}
}

func TestConsumerCreateSuccess(t *testing.T) {
	connManager, err := NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
	if err != nil {
		t.Skipf("Could not establish a connection to AMQP server: %v", err)
	}
	defer connManager.Close()

	topics := []string{"test.*"}
	consumer, err := NewConsumer(connManager, "logs_topic", topics, "test_queue", false)
	if err != nil {
		t.Errorf("Error creating Consumer: %v", err)
	}

	if !consumer.IsConnected() {
		t.Error("Consumer should be connected")
	}
}

func TestRetryWithBackoff(t *testing.T) {
	config := DefaultConnectionConfig()
	logger := slog.Default()

	// Test successful operation
	err := RetryWithBackoff(nil, config, logger, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Successful operation should not return error: %v", err)
	}

	// Test operation that fails with connection error
	attempts := 0
	err = RetryWithBackoff(nil, config, logger, func() error {
		attempts++
		if attempts < 3 {
			return amqp.ErrClosed
		}
		return nil
	})
	if err != nil {
		t.Errorf("Operation that succeeds after retries should not return error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}
