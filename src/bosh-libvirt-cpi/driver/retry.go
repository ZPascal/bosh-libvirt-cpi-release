package driver

import (
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type RetryableError interface {
	Retryable()
}

type RetryableErrorImpl struct {
	Err error
}

func (RetryableErrorImpl) Retryable()      {}
func (e RetryableErrorImpl) Error() string { return e.Err.Error() }

type Retrier interface {
	Retry(func() error) error
	RetryComplex(func() error, int, time.Duration) error
}

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

// AttemptsWithDelay attempts to execute the given function multiple times with a delay between attempts
func AttemptsWithDelay(attempts int, delay time.Duration, fn func() error, logger boshlog.Logger) error {
	var lastErr error
	
	for i := 0; i < attempts; i++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		
		if i < attempts-1 {
			if delay > 0 {
				time.Sleep(delay)
			}
		}
	}
	
	return bosherr.WrapErrorf(lastErr, "Retried '%d' times", attempts)
}

