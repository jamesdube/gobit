# Gobit Client

A simple Rabbitmq client written in go

## Getting Started

```shell
    go get "github.com/jamesdube/gobit/"
```


### Usage

This is a publisher example

```go

package main

import (
"fmt"
"github.com/jamesdube/gobit/lib/event"
amqp "github.com/rabbitmq/amqp091-go"
"log/slog"
)

func main() {
	logger := slog.Default()
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}

	emitter, err := event.NewEventEmitter(conn)
	if err != nil {
		panic(err)
	}

	for i := 1; i < 10; i++ {
		err := emitter.Publish("sms", "sms.econet", fmt.Sprintf("message number [%d]", i))
		if err != nil {
			return
		}
		logger.Info("published message", "id", i)
	}
}
```

This is a consumer example

```go

package main

import (
"github.com/jamesdube/gobit/lib/event"
amqp "github.com/rabbitmq/amqp091-go"
"log/slog"
)
var logger = slog.Default()

func main() {

	connection, err := amqp.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}
	defer connection.Close()

	topics := []string{"sms.econet", "sms.netone"}

	consumer, err := event.NewConsumer(connection,"sms", topics, "sms", true)
	if err != nil {
		panic(err)
	}

	err = consumer.Listen(onMessageReceived)
	if err != nil {
		logger.Error("consumer error", "error", err.Error())
		return
	}
}

func onMessageReceived(b []byte) error {
	logger.Info("received message", "message", string(b))
	return nil
}

```

## Contributing

Please read [CONTRIBUTING.md](https://gist.github.com/PurpleBooth/b24679402957c63ec426) for details on our code of conduct, and the process for submitting pull requests to us.

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/your/project/tags). 


