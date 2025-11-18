# RabbitMQ Client with Connection Reconnection and Retry Mechanism

This library provides a robust RabbitMQ client with automatic connection reconnection and retry mechanisms to handle network failures and connection drops.

## Features

- **Automatic Connection Reconnection**: Automatically reconnects when the connection is lost
- **Retry with Exponential Backoff**: Retries failed operations with configurable exponential backoff
- **Connection Health Monitoring**: Background monitoring of connection state
- **Graceful Degradation**: Handles temporary network issues without failing immediately
- **Configurable Parameters**: Customizable retry intervals, max retries, and delays
- **Structured Logging**: Built-in logging with slog
- **Context Support**: Proper context handling for cancellation

## Installation

```bash
go get github.com/jamesdube/gobit
```

## Quick Start

### Basic Usage

```go
package main

import (
    "log/slog"
    "github.com/jamesdube/gobit/lib/event"
)

func main() {
    // Create connection manager
    connManager, err := event.NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
    if err != nil {
        panic(err)
    }
    defer connManager.Close()

    // Create emitter
    emitter, err := event.NewEventEmitter(connManager)
    if err != nil {
        panic(err)
    }
    defer emitter.Close()

    // Publish message
    err = emitter.Publish("logs_topic", "notifications.example", `{"message": "Hello World!"}`)
    if err != nil {
        panic(err)
    }

    // Create consumer
    topics := []string{"notifications.*"}
    consumer, err := event.NewConsumer(connManager, "logs_topic", topics, "my_queue", true)
    if err != nil {
        panic(err)
    }
    defer consumer.Close()

    // Listen for messages
    err = consumer.Listen(func(data []byte) error {
        slog.Info("Received message", "data", string(data))
        return nil
    })
}
```

### Advanced Configuration

```go
package main

import (
    "log/slog"
    "time"
    "github.com/jamesdube/gobit/lib/event"
)

func main() {
    // Custom configuration
    config := event.ConnectionConfig{
        MaxRetries:        10,
        RetryInterval:     2 * time.Second,
        MaxRetryInterval:  60 * time.Second,
        ReconnectDelay:    5 * time.Second,
        HeartbeatInterval: 15 * time.Second,
    }

    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Create connection manager with custom config
    connManager, err := event.NewConnectionManagerFromURLWithConfig(
        "amqp://guest:guest@localhost:5672",
        config,
        logger,
    )
    if err != nil {
        panic(err)
    }
    defer connManager.Close()

    // Create emitter with custom config
    emitter, err := event.NewEventEmitterWithConfig(connManager, config, logger)
    if err != nil {
        panic(err)
    }
    defer emitter.Close()

    // Use emitter...
}
```

## API Reference

### ConnectionManager

Manages RabbitMQ connections with auto-reconnection.

#### Methods

- `NewConnectionManager(url string, config ConnectionConfig, logger *slog.Logger) (*ConnectionManager, error)`
- `NewConnectionManagerFromURL(url string) (*ConnectionManager, error)`
- `NewConnectionManagerFromURLWithConfig(url string, config ConnectionConfig, logger *slog.Logger) (*ConnectionManager, error)`
- `GetConnection() (*amqp.Connection, error)`
- `GetChannel() (*amqp.Channel, error)`
- `IsConnected() bool`
- `TriggerReconnect()`
- `Close() error`

### Emitter

Publishes messages to RabbitMQ exchanges with retry logic.

#### Methods

- `NewEventEmitter(connManager *ConnectionManager) (Emitter, error)`
- `NewEventEmitterWithConfig(connManager *ConnectionManager, config ConnectionConfig, logger *slog.Logger) (Emitter, error)`
- `Publish(exchange string, topic string, message string) error`
- `IsConnected() bool`
- `Close() error`

### Consumer

Consumes messages from RabbitMQ queues with auto-reconnection.

#### Methods

- `NewConsumer(connManager *ConnectionManager, exchange string, topics []string, queue string, durable bool) (Consumer, error)`
- `NewConsumerWithConfig(connManager *ConnectionManager, exchange string, topics []string, queue string, durable bool, config ConnectionConfig, logger *slog.Logger) (Consumer, error)`
- `Listen(f func(b []byte) error) error`
- `SetPrefetchCount(count int)`
- `SetLogger(logger *slog.Logger)`
- `Stop()`
- `IsConnected() bool`
- `IsRunning() bool`
- `Close() error`

### Configuration

#### ConnectionConfig

```go
type ConnectionConfig struct {
    MaxRetries        int           // Maximum number of retry attempts
    RetryInterval     time.Duration // Initial retry interval
    MaxRetryInterval  time.Duration // Maximum retry interval
    ReconnectDelay    time.Duration // Delay between reconnection attempts
    HeartbeatInterval time.Duration // Connection monitoring interval
}
```

#### DefaultConnectionConfig()

Returns default configuration suitable for most use cases:

```go
ConnectionConfig{
    MaxRetries:        5,
    RetryInterval:     1 * time.Second,
    MaxRetryInterval:  30 * time.Second,
    ReconnectDelay:    5 * time.Second,
    HeartbeatInterval: 10 * time.Second,
}
```

## Error Handling

The library automatically handles connection-related errors:

- **Connection Errors**: Automatically triggers reconnection
- **Channel Errors**: Creates new channels and retries operations
- **Network Issues**: Implements exponential backoff for retries

Connection-related errors include:
- "channel/connection is not open"
- "connection closed"
- "channel closed"
- "EOF"
- "connection refused"
- "network is unreachable"
- "no such host"

## Migration from Previous Version

### Before (v1.x)

```go
// Old way - no reconnection
conn, err := amqp.Dial("amqp://guest:guest@localhost:5672")
if err != nil {
    panic(err)
}

emitter, err := NewEventEmitter(conn)
if err != nil {
    panic(err)
}

err = emitter.Publish("logs_topic", "test", "message")
// This would fail if connection was lost
```

### After (v2.x)

```go
// New way - with reconnection and retry
connManager, err := NewConnectionManagerFromURL("amqp://guest:guest@localhost:5672")
if err != nil {
    panic(err)
}

emitter, err := NewEventEmitter(connManager)
if err != nil {
    panic(err)
}

err = emitter.Publish("logs_topic", "test", "message")
// This will automatically retry and reconnect if needed
```

## Testing

Run the tests:

```bash
go test ./lib/event/...
```

Note: Some tests require a running RabbitMQ instance. Tests will be skipped if RabbitMQ is not available.

## Example

See the `example/` directory for a complete working example demonstrating:

- Connection management
- Message publishing with retry
- Message consumption with reconnection
- Graceful shutdown

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the tests
6. Submit a pull request

## License

This project is licensed under the MIT License.