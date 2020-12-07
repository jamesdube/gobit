# Gobit Client

A simple Rabbitmq client written in go

## Getting Started

```shell
    go get github.com/jamesdube/gobit/
```


### Usage

This is an example

```go

package main

import (
	"github.com/jamesdube/gobit/lib/event"
	"github.com/streadway/amqp"
	"log"
)

func main() {

	connection, err := amqp.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		panic(err)
	}
	defer connection.Close()

	topics := []string{"my-topic"}

	consumer, err := event.NewConsumer("my-exchange",topics,"info",connection)
	if err != nil {
		panic(err)
	}
	consumer.Listen(func(b []byte) {
		log.Printf("Received a message: %s", b)
	})
}

```

## Contributing

Please read [CONTRIBUTING.md](https://gist.github.com/PurpleBooth/b24679402957c63ec426) for details on our code of conduct, and the process for submitting pull requests to us.

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/your/project/tags). 


