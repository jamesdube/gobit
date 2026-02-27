package event

import "errors"

// PermanentError signals that a message should be sent to the DLQ immediately
// and never retried. Use this for unrecoverable errors like malformed payloads.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// NewPermanentError wraps an error as permanent (no retry, send to DLQ).
func NewPermanentError(err error) error {
	return &PermanentError{Err: err}
}

// TransientError signals that a message should be retried with backoff.
// Use this for recoverable errors like temporary network failures.
type TransientError struct {
	Err error
}

func (e *TransientError) Error() string { return e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

// NewTransientError wraps an error as transient (retry with backoff).
func NewTransientError(err error) error {
	return &TransientError{Err: err}
}

// IsPermanent returns true if the error is a PermanentError.
func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}

// IsTransient returns true if the error is a TransientError.
func IsTransient(err error) bool {
	var te *TransientError
	return errors.As(err, &te)
}
