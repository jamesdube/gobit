package event

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Emitter for publishing AMQP events
type Emitter struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

func (e *Emitter) setup() error {
	channel, err := e.connection.Channel()
	e.channel = channel
	if err != nil {
		panic(err)
	}

	defer channel.Close()
	return declareExchange(channel, "logs_topic")
}

// Push (Publish) a specified message to the AMQP exchange

func (e *Emitter) Publish(exchange string, topic string, message string) error {
	channel := e.channel
	defer channel.Close()

	err := channel.PublishWithContext(
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
	//log.Printf("Sending message: %s -> %s", message, exchange)
	return err
}

// NewEventEmitter returns a new event.Emitter object
// ensuring that the object is initialised, without error
func NewEventEmitter(conn *amqp.Connection) (Emitter, error) {
	emitter := Emitter{
		connection: conn,
	}

	err := emitter.setup()
	if err != nil {
		return Emitter{}, err
	}

	return emitter, nil
}
