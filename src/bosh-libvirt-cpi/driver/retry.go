package driver

import (
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

// RetryableError marks an error as safe to retry.
type RetryableError interface {
	Retryable()
}

// RetryableErrorImpl wraps an error to mark it retryable.
type RetryableErrorImpl struct {
	Err error
}

func (RetryableErrorImpl) Retryable()      {}
func (e RetryableErrorImpl) Error() string { return e.Err.Error() }

// Retrier retries a function until it succeeds or exhausts attempts.
type Retrier interface {
	Retry(func() error) error
	RetryComplex(func() error, int, time.Duration) error
}

// RetrierImpl is the default Retrier implementation.
type RetrierImpl struct{}

func (r RetrierImpl) Retry(actionFunc func() error) error {
	return r.RetryComplex(actionFunc, 30, 2*time.Second)
}

func (RetrierImpl) RetryComplex(actionFunc func() error, times int, sleep time.Duration) error {
	var lastErr error

	for i := 0; i < times; i++ {
		lastErr = actionFunc()
		if lastErr == nil {
			return nil
		}

		if _, ok := lastErr.(RetryableError); !ok {
			return bosherr.WrapError(lastErr, "Encountered non-retryable error")
		}

		time.Sleep(sleep)
	}

	return bosherr.WrapErrorf(lastErr, "Retried '%d' times", times)
}
