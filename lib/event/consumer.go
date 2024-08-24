package event

import (
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
)

// Consumer for receiving AMPQ events
type Consumer struct {
	conn          *amqp.Connection
	exchangeName  string
	topics        []string
	queueName     string
	durable       bool
	prefetchCount int
	logger        *slog.Logger
}

var logger = slog.Default()

func (consumer *Consumer) setup() error {
	channel, err := consumer.conn.Channel()
	if err != nil {
		return err
	}
	return declareExchange(channel, consumer.exchangeName)
}

// NewConsumer returns a new Consumer
func NewConsumer(conn *amqp.Connection, exchange string, topics []string, queue string, durable bool) (Consumer, error) {
	consumer := Consumer{
		conn: conn, exchangeName: exchange, topics: topics, queueName: queue, durable: durable,
	}
	err := consumer.setup()
	if err != nil {
		return Consumer{}, err
	}

	return consumer, nil
}

func (consumer *Consumer) SetPrefetchCount(count int) {
	consumer.prefetchCount = count
}

func (consumer *Consumer) SetLogger(logger *slog.Logger) {
	consumer.logger = logger
}

// Listen will listen for all new Queue publications
// and print them to the console.
func (consumer *Consumer) Listen(f func(b []byte) error) error {

	if consumer.logger == nil {
		consumer.logger = slog.Default()
	}

	ch, err := consumer.conn.Channel()
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

	forever := make(chan bool)

	consumer.logger.Debug("started rabbitmq consumer.", "tag", tag, "exchange", consumer.exchangeName, "queue", q.Name)

	go func() {
		for delivery := range msgs {
			err := f(delivery.Body)
			if err != nil {
				err := delivery.Nack(true, true)
				if err != nil {
					logger.Error("error on ack", "error", err.Error())
					return
				}
				return
			}

			err = delivery.Ack(true)
			if err != nil {
				consumer.logger.Error("error on ack", "error", err.Error())
				return
			}
		}
	}()
	<-forever
	return nil
}
