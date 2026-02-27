package event

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewPermanentError(t *testing.T) {
	inner := errors.New("bad payload")
	err := NewPermanentError(inner)

	if err.Error() != "bad payload" {
		t.Errorf("expected 'bad payload', got %q", err.Error())
	}

	if !IsPermanent(err) {
		t.Error("expected IsPermanent to return true")
	}

	if IsTransient(err) {
		t.Error("expected IsTransient to return false for PermanentError")
	}
}

func TestNewTransientError(t *testing.T) {
	inner := errors.New("timeout")
	err := NewTransientError(inner)

	if err.Error() != "timeout" {
		t.Errorf("expected 'timeout', got %q", err.Error())
	}

	if !IsTransient(err) {
		t.Error("expected IsTransient to return true")
	}

	if IsPermanent(err) {
		t.Error("expected IsPermanent to return false for TransientError")
	}
}

func TestUnclassifiedErrorIsNeitherPermanentNorTransient(t *testing.T) {
	err := errors.New("something went wrong")

	if IsPermanent(err) {
		t.Error("unclassified error should not be permanent")
	}
	if IsTransient(err) {
		t.Error("unclassified error should not be transient")
	}
}

func TestNilErrorIsNeitherPermanentNorTransient(t *testing.T) {
	if IsPermanent(nil) {
		t.Error("nil should not be permanent")
	}
	if IsTransient(nil) {
		t.Error("nil should not be transient")
	}
}

func TestPermanentErrorUnwrap(t *testing.T) {
	inner := errors.New("root cause")
	err := NewPermanentError(inner)

	if !errors.Is(err, inner) {
		t.Error("expected errors.Is to find inner error via Unwrap")
	}
}

func TestTransientErrorUnwrap(t *testing.T) {
	inner := errors.New("root cause")
	err := NewTransientError(inner)

	if !errors.Is(err, inner) {
		t.Error("expected errors.Is to find inner error via Unwrap")
	}
}

func TestWrappedPermanentError(t *testing.T) {
	inner := NewPermanentError(errors.New("bad data"))
	wrapped := fmt.Errorf("handler failed: %w", inner)

	if !IsPermanent(wrapped) {
		t.Error("expected IsPermanent to find PermanentError through wrapping")
	}
}

func TestWrappedTransientError(t *testing.T) {
	inner := NewTransientError(errors.New("connection reset"))
	wrapped := fmt.Errorf("handler failed: %w", inner)

	if !IsTransient(wrapped) {
		t.Error("expected IsTransient to find TransientError through wrapping")
	}
}

func TestGetRetryCount(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]interface{}
		expected int
	}{
		{"nil headers", nil, 0},
		{"no retry header", map[string]interface{}{"other": "val"}, 0},
		{"int32 value", map[string]interface{}{"x-retry-count": int32(3)}, 3},
		{"int64 value", map[string]interface{}{"x-retry-count": int64(7)}, 7},
		{"int value", map[string]interface{}{"x-retry-count": int(2)}, 2},
		{"string value fallback", map[string]interface{}{"x-retry-count": "bad"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily construct amqp.Delivery in unit tests without the full
			// library, but getRetryCount just reads from Headers which is amqp.Table
			// (map[string]interface{}). We test the helper via the exported types.
			headers := make(map[string]interface{})
			for k, v := range tt.headers {
				headers[k] = v
			}
			// Direct test of header parsing logic
			count := retryCountFromHeaders(headers)
			if count != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, count)
			}
		})
	}
}

func TestDefaultConsumerConfig(t *testing.T) {
	cfg := DefaultConsumerConfig()

	if cfg.PrefetchCount != 50 {
		t.Errorf("expected PrefetchCount 50, got %d", cfg.PrefetchCount)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", cfg.MaxRetries)
	}
	if !cfg.EnableDLQ {
		t.Error("expected EnableDLQ to be true")
	}
	if cfg.RetryDelay != 2*1e9 {
		t.Errorf("expected RetryDelay 2s, got %v", cfg.RetryDelay)
	}
}

func TestCopyHeaders(t *testing.T) {
	original := map[string]interface{}{
		"key1": "val1",
		"key2": int32(42),
	}

	copied := copyHeaders(original)

	// Verify values are copied
	if copied["key1"] != "val1" {
		t.Error("expected key1 to be copied")
	}
	if copied["key2"] != int32(42) {
		t.Error("expected key2 to be copied")
	}

	// Verify independence — modifying copy doesn't affect original
	copied["key3"] = "new"
	if _, exists := original["key3"]; exists {
		t.Error("modifying copy should not affect original")
	}
}

func TestCopyHeadersNil(t *testing.T) {
	copied := copyHeaders(nil)
	if copied == nil {
		t.Error("copyHeaders(nil) should return non-nil map")
	}
}
