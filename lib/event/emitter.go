package event

import (
	"context"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
)

// Emitter for publishing AMQP events
type Emitter struct {
	connManager *ConnectionManager
	config      ConnectionConfig
	logger      *slog.Logger
	mu          sync.Mutex
	ch          *amqp.Channel
	exchanges   map[string]bool // tracks declared exchanges on current channel
}

func (e *Emitter) setup() error {
	// Validate that the connection manager can provide a channel
	ch, err := e.connManager.GetChannel()
	if err != nil {
		return err
	}
	e.ch = ch
	return nil
}

// getChannel returns the persistent channel, creating a new one if needed.
func (e *Emitter) getChannel() (*amqp.Channel, error) {
	if e.ch != nil && !e.ch.IsClosed() {
		return e.ch, nil
	}
	// Channel is nil or closed — get a fresh one
	ch, err := e.connManager.GetChannel()
	if err != nil {
		return nil, err
	}
	e.ch = ch
	e.exchanges = make(map[string]bool) // reset declared exchanges
	return ch, nil
}

// Publish a message to the AMQP exchange with retry logic.
func (e *Emitter) Publish(exchange string, topic string, message string) error {
	return RetryWithBackoff(context.Background(), e.config, e.logger, func() error {
		e.mu.Lock()
		defer e.mu.Unlock()

		ch, err := e.getChannel()
		if err != nil {
			return err
		}

		// Declare exchange once per channel lifetime
		if !e.exchanges[exchange] {
			if err := declareExchange(ch, exchange); err != nil {
				// Channel may be broken — force reset on next call
				e.ch = nil
				return err
			}
			e.exchanges[exchange] = true
		}

		err = ch.PublishWithContext(
			context.Background(),
			exchange,
			topic,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         []byte(message),
				DeliveryMode: 2,
			},
		)
		if err != nil {
			// Channel may be broken — force reset on next call
			e.ch = nil
			return err
		}
		return nil
	})
}

// NewEventEmitter returns a new event.Emitter object
// ensuring that the object is initialised, without error
func NewEventEmitter(connManager *ConnectionManager) (Emitter, error) {
	emitter := Emitter{
		connManager: connManager,
		config:      DefaultConnectionConfig(),
		logger:      slog.Default(),
		exchanges:   make(map[string]bool),
	}

	err := emitter.setup()
	if err != nil {
		return Emitter{}, err
	}

	return emitter, nil
}

// NewEventEmitterWithConfig returns a new event.Emitter object with custom configuration
func NewEventEmitterWithConfig(connManager *ConnectionManager, config ConnectionConfig, logger *slog.Logger) (Emitter, error) {
	if logger == nil {
		logger = slog.Default()
	}

	emitter := Emitter{
		connManager: connManager,
		config:      config,
		logger:      logger,
		exchanges:   make(map[string]bool),
	}

	err := emitter.setup()
	if err != nil {
		return Emitter{}, err
	}

	return emitter, nil
}

// Close closes the emitter's channel and its connection manager
func (e *Emitter) Close() error {
	e.mu.Lock()
	if e.ch != nil && !e.ch.IsClosed() {
		e.ch.Close()
	}
	e.mu.Unlock()
	return e.connManager.Close()
}

// IsConnected returns true if the emitter is connected to RabbitMQ
func (e *Emitter) IsConnected() bool {
	return e.connManager.IsConnected()
}
