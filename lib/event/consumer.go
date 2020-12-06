package event

import (
	"log"

	"github.com/streadway/amqp"
)

// Consumer for receiving AMPQ events
type Consumer struct {
	conn      		*amqp.Connection
	exchangeName 	string
	topics			[]string
	queueName 		string
}

func (consumer *Consumer) setup() error {
	channel, err := consumer.conn.Channel()
	if err != nil {
		return err
	}
	return declareExchange(channel,consumer.exchangeName)
}

// NewConsumer returns a new Consumer
func NewConsumer(exchange string, topics []string,queue string,conn *amqp.Connection) (Consumer, error) {
	consumer := Consumer{
		conn: conn,exchangeName: exchange,topics: topics,queueName: queue,
	}
	err := consumer.setup()
	if err != nil {
		return Consumer{}, err
	}

	return consumer, nil
}

// Listen will listen for all new Queue publications
// and print them to the console.
func (consumer *Consumer) Listen(f func(b []byte)) error {
	ch, err := consumer.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := declareQueue(ch,consumer.queueName)
	if err != nil {
		return err
	}

	for _, s := range consumer.topics {
		err = ch.QueueBind(
			q.Name,
			s,
			consumer.exchangeName,
			false,
			nil,
		)

		if err != nil {
			return err
		}
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return err
	}

	forever := make(chan bool)
	go func() {
		for m := range msgs {
			f(m.Body)

		}
	}()

	log.Printf("[*] Waiting for message [Exchange, Queue][%s, %s]. To exit press CTRL+C", getExchangeName(), q.Name)
	<-forever
	return nil
}
