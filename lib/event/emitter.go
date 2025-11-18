package event

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
)

// Emitter for publishing AMQP events
type Emitter struct {
	connManager *ConnectionManager
	config      ConnectionConfig
	logger      *slog.Logger
}

func (e *Emitter) setup() error {
	channel, err := e.connManager.GetChannel()
	if err != nil {
		return err
	}
	defer channel.Close()

	return declareExchange(channel, "logs_topic")
}

// Push (Publish) a specified message to the AMQP exchange with retry logic
func (e *Emitter) Publish(exchange string, topic string, message string) error {
	return RetryWithBackoff(context.Background(), e.config, e.logger, func() error {
		channel, err := e.connManager.GetChannel()
		if err != nil {
			return err
		}
		defer channel.Close()

		// Ensure exchange exists
		err = declareExchange(channel, exchange)
		if err != nil {
			return err
		}

		return channel.PublishWithContext(
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
	})
}

// NewEventEmitter returns a new event.Emitter object
// ensuring that the object is initialised, without error
func NewEventEmitter(connManager *ConnectionManager) (Emitter, error) {
	emitter := Emitter{
		connManager: connManager,
		config:      DefaultConnectionConfig(),
		logger:      slog.Default(),
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
	}

	err := emitter.setup()
	if err != nil {
		return Emitter{}, err
	}

	return emitter, nil
}

// Close closes the emitter and its connection manager
func (e *Emitter) Close() error {
	return e.connManager.Close()
}

// IsConnected returns true if the emitter is connected to RabbitMQ
func (e *Emitter) IsConnected() bool {
	return e.connManager.IsConnected()
}
