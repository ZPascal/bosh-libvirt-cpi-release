package driver

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryableError(t *testing.T) {
	err := errors.New("test error")
	retryErr := RetryableErrorImpl{Err: err}

	assert.Equal(t, "test error", retryErr.Error())
}

func TestRetrierImplSuccessFirstAttempt(t *testing.T) {
	retrier := RetrierImpl{}
	attempts := 0

	err := retrier.Retry(func() error {
		attempts++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetrierImplRetryableError(t *testing.T) {
	retrier := RetrierImpl{}
	attempts := 0

	err := retrier.RetryComplex(func() error {
		attempts++
		if attempts < 3 {
			return RetryableErrorImpl{Err: errors.New("retryable")}
		}
		return nil
	}, 5, 1*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetrierImplNonRetryableError(t *testing.T) {
	retrier := RetrierImpl{}
	attempts := 0

	err := retrier.RetryComplex(func() error {
		attempts++
		return errors.New("non-retryable")
	}, 3, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetrierImplExhaustsRetries(t *testing.T) {
	retrier := RetrierImpl{}
	attempts := 0

	err := retrier.RetryComplex(func() error {
		attempts++
		return RetryableErrorImpl{Err: errors.New("always fails")}
	}, 3, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 3, attempts)
}

func TestAttemptsWithDelaySuccess(t *testing.T) {
	attempts := 0
	err := AttemptsWithDelay(3, 1*time.Millisecond, func() error {
		attempts++
		return nil
	}, nil)

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestAttemptsWithDelayRetry(t *testing.T) {
	attempts := 0
	err := AttemptsWithDelay(3, 1*time.Millisecond, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("fail")
		}
		return nil
	}, nil)

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestAttemptsWithDelayExhausts(t *testing.T) {
	attempts := 0
	err := AttemptsWithDelay(2, 1*time.Millisecond, func() error {
		attempts++
		return errors.New("always fails")
	}, nil)

	assert.Error(t, err)
	assert.Equal(t, 2, attempts)
}

func TestAttemptsWithDelayZeroDelay(t *testing.T) {
	attempts := 0
	err := AttemptsWithDelay(3, 0, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("fail")
		}
		return nil
	}, nil)

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}
