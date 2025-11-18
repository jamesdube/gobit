package event

import (
	"context"
	"errors"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	"sync"
	"time"
)

// Consumer for receiving AMPQ events
type Consumer struct {
	connManager   *ConnectionManager
	exchangeName  string
	topics        []string
	queueName     string
	durable       bool
	prefetchCount int
	logger        *slog.Logger
	config        ConnectionConfig
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	running       bool
}

var logger = slog.Default()

func (consumer *Consumer) setup() error {
	channel, err := consumer.connManager.GetChannel()
	if err != nil {
		return err
	}
	defer channel.Close()
	return declareExchange(channel, consumer.exchangeName)
}

// NewConsumer returns a new Consumer
func NewConsumer(connManager *ConnectionManager, exchange string, topics []string, queue string, durable bool) (Consumer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	consumer := Consumer{
		connManager:  connManager,
		exchangeName: exchange,
		topics:       topics,
		queueName:    queue,
		durable:      durable,
		logger:       slog.Default(),
		config:       DefaultConnectionConfig(),
		ctx:          ctx,
		cancel:       cancel,
	}

	err := consumer.setup()
	if err != nil {
		cancel()
		return Consumer{}, err
	}

	return consumer, nil
}

// NewConsumerWithConfig returns a new Consumer with custom configuration
func NewConsumerWithConfig(connManager *ConnectionManager, exchange string, topics []string, queue string, durable bool, config ConnectionConfig, logger *slog.Logger) (Consumer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	consumer := Consumer{
		connManager:  connManager,
		exchangeName: exchange,
		topics:       topics,
		queueName:    queue,
		durable:      durable,
		logger:       logger,
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
	}

	err := consumer.setup()
	if err != nil {
		cancel()
		return Consumer{}, err
	}

	return consumer, nil
}

func (consumer *Consumer) SetPrefetchCount(count int) {
	consumer.mu.Lock()
	defer consumer.mu.Unlock()
	consumer.prefetchCount = count
}

func (consumer *Consumer) SetLogger(logger *slog.Logger) {
	consumer.mu.Lock()
	defer consumer.mu.Unlock()
	consumer.logger = logger
}

// Listen will listen for all new Queue publications with auto-reconnection
func (consumer *Consumer) Listen(f func(b []byte) error) error {
	consumer.mu.Lock()
	if consumer.running {
		consumer.mu.Unlock()
		return errors.New("consumer is already running")
	}
	consumer.running = true
	consumer.mu.Unlock()

	defer func() {
		consumer.mu.Lock()
		consumer.running = false
		consumer.mu.Unlock()
	}()

	if consumer.logger == nil {
		consumer.logger = slog.Default()
	}

	return consumer.listenWithReconnect(f)
}

// listenWithReconnect handles the actual listening with reconnection logic
func (consumer *Consumer) listenWithReconnect(f func(b []byte) error) error {
	reconnectDelay := consumer.config.ReconnectDelay

	for {
		select {
		case <-consumer.ctx.Done():
			return consumer.ctx.Err()
		default:
		}

		err := consumer.listenOnce(f)
		if err != nil {
			consumer.logger.Error("Consumer connection lost", "error", err.Error())

			// Wait before reconnecting
			select {
			case <-consumer.ctx.Done():
				return consumer.ctx.Err()
			case <-time.After(reconnectDelay):
			}

			// Exponential backoff for reconnection delay
			reconnectDelay = time.Duration(float64(reconnectDelay) * 1.5)
			if reconnectDelay > consumer.config.MaxRetryInterval {
				reconnectDelay = consumer.config.MaxRetryInterval
			}

			continue
		}

		// If we reach here, the connection was closed gracefully
		return nil
	}
}

// listenOnce sets up the consumer and listens for messages once
func (consumer *Consumer) listenOnce(f func(b []byte) error) error {
	// Wait for connection to be available
	for !consumer.connManager.IsConnected() {
		select {
		case <-consumer.ctx.Done():
			return consumer.ctx.Err()
		case <-time.After(1 * time.Second):
			consumer.logger.Debug("Waiting for connection to be available...")
		}
	}

	ch, err := consumer.connManager.GetChannel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := declareQueue(ch, consumer.queueName, consumer.durable)
	if err != nil {
		return err
	}

	for _, topic := range consumer.topics {
		err = ch.QueueBind(
			q.Name,
			topic,
			consumer.exchangeName,
			false,
			nil,
		)

		if err != nil {
			return err
		}
	}

	err = ch.Qos(consumer.prefetchCount, 0, false)
	if err != nil {
		return err
	}

	tag := uuid.NewString()
	msgs, err := ch.Consume(q.Name, tag, false, false, false, false, nil)
	if err != nil {
		return err
	}

	consumer.logger.Debug("started rabbitmq consumer.", "tag", tag, "exchange", consumer.exchangeName, "queue", q.Name)

	// Handle connection close notification
	connCloseChan := consumer.connManager.conn.NotifyClose(make(chan *amqp.Error, 1))
	chCloseChan := ch.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-consumer.ctx.Done():
			return consumer.ctx.Err()
		case delivery, ok := <-msgs:
			if !ok {
				return errors.New("message channel closed")
			}

			err := f(delivery.Body)
			if err != nil {
				err := delivery.Nack(true, true)
				if err != nil {
					consumer.logger.Error("error on nack", "error", err.Error())
					return err
				}
				return err
			}

			err = delivery.Ack(true)
			if err != nil {
				consumer.logger.Error("error on ack", "error", err.Error())
				return err
			}

		case <-connCloseChan:
			return errors.New("connection closed")

		case <-chCloseChan:
			return errors.New("channel closed")
		}
	}
}

// Stop stops the consumer
func (consumer *Consumer) Stop() {
	consumer.mu.Lock()
	defer consumer.mu.Unlock()

	if consumer.cancel != nil {
		consumer.cancel()
	}
}

// Close closes the consumer and its connection manager
func (consumer *Consumer) Close() error {
	consumer.Stop()
	return consumer.connManager.Close()
}

// IsConnected returns true if the consumer is connected to RabbitMQ
func (consumer *Consumer) IsConnected() bool {
	return consumer.connManager.IsConnected()
}

// IsRunning returns true if the consumer is currently listening
func (consumer *Consumer) IsRunning() bool {
	consumer.mu.RLock()
	defer consumer.mu.RUnlock()
	return consumer.running
}
