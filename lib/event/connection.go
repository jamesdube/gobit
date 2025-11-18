package event

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrConnectionClosed = errors.New("connection is closed")
	ErrChannelClosed    = errors.New("channel is closed")
)

// ConnectionConfig holds configuration for connection management
type ConnectionConfig struct {
	MaxRetries        int
	RetryInterval     time.Duration
	MaxRetryInterval  time.Duration
	ReconnectDelay    time.Duration
	HeartbeatInterval time.Duration
}

// DefaultConnectionConfig returns default configuration
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		MaxRetries:        5,
		RetryInterval:     1 * time.Second,
		MaxRetryInterval:  30 * time.Second,
		ReconnectDelay:    5 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}
}

// ConnectionManager manages RabbitMQ connections with auto-reconnection
type ConnectionManager struct {
	conn          *amqp.Connection
	connectionURL string
	config        ConnectionConfig
	logger        *slog.Logger
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	reconnecting  bool
	reconnectChan chan struct{}
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(url string, config ConnectionConfig, logger *slog.Logger) (*ConnectionManager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	cm := &ConnectionManager{
		config:        config,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		reconnectChan: make(chan struct{}, 1),
	}

	// Initial connection
	conn, err := amqp.Dial(url)
	if err != nil {
		cancel()
		return nil, err
	}
	cm.conn = conn

	// Store the URL for reconnection
	cm.connectionURL = url

	// Start connection monitoring
	go cm.monitorConnection()

	return cm, nil
}

// GetConnection returns a valid connection
func (cm *ConnectionManager) GetConnection() (*amqp.Connection, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.conn == nil || cm.conn.IsClosed() {
		return nil, ErrConnectionClosed
	}

	return cm.conn, nil
}

// GetChannel returns a valid channel, creating one if needed
func (cm *ConnectionManager) GetChannel() (*amqp.Channel, error) {
	conn, err := cm.GetConnection()
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return ch, nil
}

// monitorConnection monitors the connection and triggers reconnection
func (cm *ConnectionManager) monitorConnection() {
	ticker := time.NewTicker(cm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.mu.RLock()
			conn := cm.conn
			cm.mu.RUnlock()

			if conn == nil || conn.IsClosed() {
				if !cm.reconnecting {
					cm.logger.Warn("Connection lost, attempting to reconnect...")
					go cm.reconnect()
				}
			}
		case <-cm.reconnectChan:
			if !cm.reconnecting {
				cm.logger.Warn("Reconnection triggered...")
				go cm.reconnect()
			}
		}
	}
}

// reconnect attempts to reconnect to RabbitMQ
func (cm *ConnectionManager) reconnect() {
	cm.mu.Lock()
	if cm.reconnecting {
		cm.mu.Unlock()
		return
	}
	cm.reconnecting = true
	cm.mu.Unlock()

	defer func() {
		cm.mu.Lock()
		cm.reconnecting = false
		cm.mu.Unlock()
	}()

	retryInterval := cm.config.RetryInterval

	for i := 0; i < cm.config.MaxRetries; i++ {
		select {
		case <-cm.ctx.Done():
			return
		default:
		}

		cm.logger.Info("Attempting to reconnect...", "attempt", i+1, "maxRetries", cm.config.MaxRetries)

		// Close existing connection if it exists
		cm.mu.Lock()
		if cm.conn != nil && !cm.conn.IsClosed() {
			cm.conn.Close()
		}
		cm.conn = nil
		cm.mu.Unlock()

		// Try to reconnect
		conn, err := amqp.Dial(cm.connectionURL)
		if err == nil {
			cm.mu.Lock()
			cm.conn = conn
			cm.mu.Unlock()

			cm.logger.Info("Successfully reconnected to RabbitMQ")
			return
		}

		cm.logger.Error("Reconnection failed", "error", err.Error(), "attempt", i+1)

		if i < cm.config.MaxRetries-1 {
			select {
			case <-cm.ctx.Done():
				return
			case <-time.After(retryInterval):
			}

			// Exponential backoff
			retryInterval = time.Duration(float64(retryInterval) * 1.5)
			if retryInterval > cm.config.MaxRetryInterval {
				retryInterval = cm.config.MaxRetryInterval
			}
		}
	}

	cm.logger.Error("Failed to reconnect after maximum retries", "maxRetries", cm.config.MaxRetries)
}

// TriggerReconnect manually triggers a reconnection
func (cm *ConnectionManager) TriggerReconnect() {
	select {
	case cm.reconnectChan <- struct{}{}:
	default:
		// Channel is full, reconnection already triggered
	}
}

// Close closes the connection and stops monitoring
func (cm *ConnectionManager) Close() error {
	cm.cancel()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.conn != nil && !cm.conn.IsClosed() {
		return cm.conn.Close()
	}

	return nil
}

// IsConnected returns true if the connection is active
func (cm *ConnectionManager) IsConnected() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.conn != nil && !cm.conn.IsClosed()
}

// RetryWithBackoff executes a function with retry logic and exponential backoff
func RetryWithBackoff(ctx context.Context, config ConnectionConfig, logger *slog.Logger, operation func() error) error {
	// Use background context if no context is provided
	if ctx == nil {
		ctx = context.Background()
	}

	retryInterval := config.RetryInterval

	for i := 0; i < config.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			return nil
		}

		// Check if it's a connection-related error
		if isConnectionError(err) {
			if i < config.MaxRetries-1 {
				logger.Warn("Operation failed due to connection error, retrying...",
					"error", err.Error(),
					"attempt", i+1,
					"maxRetries", config.MaxRetries,
					"retryInterval", retryInterval)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(retryInterval):
				}

				// Exponential backoff
				retryInterval = time.Duration(float64(retryInterval) * 1.5)
				if retryInterval > config.MaxRetryInterval {
					retryInterval = config.MaxRetryInterval
				}
				continue
			}
		}

		// Return error immediately if it's not a connection error or max retries reached
		return err
	}

	return errors.New("max retries exceeded")
}

// isConnectionError checks if an error is connection-related
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	connectionErrors := []string{
		"channel/connection is not open",
		"connection closed",
		"channel closed",
		"EOF",
		"connection refused",
		"network is unreachable",
		"no such host",
	}

	for _, connErr := range connectionErrors {
		if contains(errStr, connErr) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
