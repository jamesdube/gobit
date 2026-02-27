package event

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer for receiving AMQP events
type Consumer struct {
	connManager    *ConnectionManager
	exchangeName   string
	topics         []string
	queueName      string
	durable        bool
	prefetchCount  int
	logger         *slog.Logger
	config         ConnectionConfig
	consumerConfig ConsumerConfig
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex
	running        bool
}

// ConsumerConfig holds consumer-level configuration for retry and DLQ behaviour.
type ConsumerConfig struct {
	PrefetchCount int
	MaxRetries    int
	EnableDLQ     bool
	RetryDelay    time.Duration
}

// DefaultConsumerConfig returns sensible defaults for consumer behaviour.
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		PrefetchCount: 50,
		MaxRetries:    5,
		EnableDLQ:     true,
		RetryDelay:    2 * time.Second,
	}
}

var logger = slog.Default()

func (consumer *Consumer) setup() error {
	channel, err := consumer.connManager.GetChannel()
	if err != nil {
		return err
	}
	defer channel.Close()

	if err := declareExchange(channel, consumer.exchangeName); err != nil {
		return err
	}

	// Declare DLQ infrastructure if enabled
	if consumer.consumerConfig.EnableDLQ {
		if err := declareDLQ(channel, consumer.queueName, consumer.exchangeName); err != nil {
			return fmt.Errorf("declare DLQ for %s: %w", consumer.queueName, err)
		}
	}

	return nil
}

// NewConsumer returns a new Consumer with legacy behaviour (no DLQ, no retry tracking).
// Backward compatible — existing callers are unaffected.
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
		consumerConfig: ConsumerConfig{
			EnableDLQ: false,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	err := consumer.setup()
	if err != nil {
		cancel()
		return Consumer{}, err
	}

	return consumer, nil
}

// NewConsumerWithConfig returns a new Consumer with custom connection configuration.
// Backward compatible — existing callers are unaffected.
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
		consumerConfig: ConsumerConfig{
			EnableDLQ: false,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	err := consumer.setup()
	if err != nil {
		cancel()
		return Consumer{}, err
	}

	return consumer, nil
}

// NewConsumerWithOptions returns a new Consumer with full control over retry, DLQ, and prefetch.
func NewConsumerWithOptions(connManager *ConnectionManager, exchange string, topics []string, queue string, durable bool, cfg ConsumerConfig) (Consumer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	consumer := Consumer{
		connManager:    connManager,
		exchangeName:   exchange,
		topics:         topics,
		queueName:      queue,
		durable:        durable,
		prefetchCount:  cfg.PrefetchCount,
		logger:         slog.Default(),
		config:         DefaultConnectionConfig(),
		consumerConfig: cfg,
		ctx:            ctx,
		cancel:         cancel,
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

	// Declare queue — with or without DLQ wiring
	var q amqp.Queue
	if consumer.consumerConfig.EnableDLQ {
		dlxExchange := consumer.exchangeName + ".dlx"
		q, err = declareQueueWithDLQ(ch, consumer.queueName, consumer.durable, dlxExchange)
	} else {
		q, err = declareQueue(ch, consumer.queueName, consumer.durable)
	}
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

	prefetch := consumer.prefetchCount
	if prefetch == 0 {
		prefetch = consumer.consumerConfig.PrefetchCount
	}
	if prefetch > 0 {
		err = ch.Qos(prefetch, 0, false)
		if err != nil {
			return err
		}
	}

	tag := uuid.NewString()
	msgs, err := ch.Consume(q.Name, tag, false, false, false, false, nil)
	if err != nil {
		return err
	}

	consumer.logger.Debug("started rabbitmq consumer.", "tag", tag, "exchange", consumer.exchangeName, "queue", q.Name)

	// Handle connection close notification — use GetConnection() to avoid data race
	conn, err := consumer.connManager.GetConnection()
	if err != nil {
		return err
	}
	connCloseChan := conn.NotifyClose(make(chan *amqp.Error, 1))
	chCloseChan := ch.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-consumer.ctx.Done():
			return consumer.ctx.Err()
		case delivery, ok := <-msgs:
			if !ok {
				return errors.New("message channel closed")
			}

			consumer.handleDelivery(ch, delivery, f)

		case <-connCloseChan:
			return errors.New("connection closed")

		case <-chCloseChan:
			return errors.New("channel closed")
		}
	}
}

// handleDelivery processes a single delivery with panic recovery, error classification,
// retry tracking, and DLQ routing.
func (consumer *Consumer) handleDelivery(ch *amqp.Channel, delivery amqp.Delivery, f func([]byte) error) {
	err := consumer.safeHandle(f, delivery.Body)

	// Success — ACK this single message (multiple=false)
	if err == nil {
		if ackErr := delivery.Ack(false); ackErr != nil {
			consumer.logger.Error("error on ack", "error", ackErr.Error())
		}
		return
	}

	// DLQ disabled — legacy behaviour: NACK with requeue
	if !consumer.consumerConfig.EnableDLQ {
		if nackErr := delivery.Nack(false, true); nackErr != nil {
			consumer.logger.Error("error on nack", "error", nackErr.Error())
		}
		return
	}

	// DLQ enabled — classify the error and decide
	if IsPermanent(err) {
		consumer.logger.Warn("permanent error, routing to DLQ",
			"queue", consumer.queueName, "error", err.Error())
		consumer.publishToDLQ(ch, delivery)
		if ackErr := delivery.Ack(false); ackErr != nil {
			consumer.logger.Error("error on ack after DLQ publish", "error", ackErr.Error())
		}
		return
	}

	// Transient or unknown — check retry count
	retryCount := getRetryCount(delivery)

	if retryCount >= consumer.consumerConfig.MaxRetries {
		consumer.logger.Warn("max retries exceeded, routing to DLQ",
			"queue", consumer.queueName, "retries", retryCount, "error", err.Error())
		consumer.publishToDLQ(ch, delivery)
		if ackErr := delivery.Ack(false); ackErr != nil {
			consumer.logger.Error("error on ack after DLQ publish", "error", ackErr.Error())
		}
		return
	}

	// Retry: republish with incremented retry count, then ACK original
	consumer.logger.Warn("retrying message",
		"queue", consumer.queueName, "retry", retryCount+1,
		"max", consumer.consumerConfig.MaxRetries, "error", err.Error())

	// Exponential backoff: delay * 2^retryCount (capped at 30s)
	backoff := consumer.consumerConfig.RetryDelay * time.Duration(1<<uint(retryCount))
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	select {
	case <-consumer.ctx.Done():
		return
	case <-time.After(backoff):
	}

	consumer.republishWithRetry(ch, delivery, retryCount+1)
	if ackErr := delivery.Ack(false); ackErr != nil {
		consumer.logger.Error("error on ack after retry republish", "error", ackErr.Error())
	}
}

// safeHandle calls the handler with panic recovery. Panics become PermanentErrors.
func (consumer *Consumer) safeHandle(f func([]byte) error, body []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			consumer.logger.Error("handler panicked", "queue", consumer.queueName, "panic", r)
			err = NewPermanentError(fmt.Errorf("handler panic: %v", r))
		}
	}()
	return f(body)
}

// getRetryCount reads the x-retry-count header from a delivery.
func getRetryCount(d amqp.Delivery) int {
	return retryCountFromHeaders(d.Headers)
}

// retryCountFromHeaders extracts x-retry-count from an AMQP header table.
func retryCountFromHeaders(headers map[string]interface{}) int {
	if headers == nil {
		return 0
	}
	v, ok := headers["x-retry-count"]
	if !ok {
		return 0
	}
	// AMQP headers can be int32, int64, or int depending on client
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}

// republishWithRetry republishes a message to the same queue with an incremented retry count.
func (consumer *Consumer) republishWithRetry(ch *amqp.Channel, d amqp.Delivery, retryCount int) {
	headers := copyHeaders(d.Headers)
	headers["x-retry-count"] = int32(retryCount)

	err := ch.PublishWithContext(
		consumer.ctx,
		consumer.exchangeName,
		d.RoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  d.ContentType,
			Body:         d.Body,
			Headers:      headers,
			DeliveryMode: d.DeliveryMode,
			MessageId:    d.MessageId,
		},
	)
	if err != nil {
		consumer.logger.Error("failed to republish for retry",
			"queue", consumer.queueName, "error", err.Error())
	}
}

// publishToDLQ republishes a message to the dead-letter exchange.
func (consumer *Consumer) publishToDLQ(ch *amqp.Channel, d amqp.Delivery) {
	dlxExchange := consumer.exchangeName + ".dlx"
	dlqRoutingKey := consumer.queueName + ".dlq"

	headers := copyHeaders(d.Headers)
	headers["x-original-routing-key"] = d.RoutingKey
	headers["x-original-exchange"] = d.Exchange

	err := ch.PublishWithContext(
		consumer.ctx,
		dlxExchange,
		dlqRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  d.ContentType,
			Body:         d.Body,
			Headers:      headers,
			DeliveryMode: d.DeliveryMode,
			MessageId:    d.MessageId,
		},
	)
	if err != nil {
		consumer.logger.Error("failed to publish to DLQ",
			"queue", consumer.queueName, "error", err.Error())
	}
}

// copyHeaders returns a shallow copy of AMQP headers, creating a new table if nil.
func copyHeaders(h amqp.Table) amqp.Table {
	out := make(amqp.Table, len(h)+2)
	for k, v := range h {
		out[k] = v
	}
	return out
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
