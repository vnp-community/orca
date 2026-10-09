package usecase

import "errors"

// notRetryable marks a failure that waiting cannot fix: the dependency refused the call (credential or configuration)
// or the data it needs is gone. The outbox relay uses it to stop holding unrelated events behind such a row.
type notRetryable struct{ err error }

func (e *notRetryable) Error() string { return e.err.Error() }
func (e *notRetryable) Unwrap() error { return e.err }

// MarkNotRetryable wraps err so IsNotRetryable reports true; the wrapped chain stays visible to errors.Is/As.
func MarkNotRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &notRetryable{err: err}
}

func IsNotRetryable(err error) bool {
	var nr *notRetryable
	return errors.As(err, &nr)
}
