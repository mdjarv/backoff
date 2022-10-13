/*
Package backoff provides a function for retrying a function with exponential backoff until the
provided operation either returns nil, indicating success, or it hits an attempts limit.
*/
package backoff

import (
	"fmt"
	"time"
)

var ErrMaxAttemptsReached = fmt.Errorf("max attempts reached")

// OperationFunc should return nil on success, otherwise returns an error
type OperationFunc = func() error

// RetryFunc is executed on a failed operation execution, just before sleeping
type RetryFunc = func(error, time.Duration)

// SleepFunc can be used to replace the default time.Sleep function, for example in unit tests
type SleepFunc = func(time.Duration)

type backoff struct {
	Min      time.Duration
	Max      time.Duration
	Attempts int

	retryFunc RetryFunc
	sleepFunc SleepFunc
	current   time.Duration
	attempt   int
}

type Option func(*backoff)

// WithRetryFunc option is used to set a function to be executed before sleeping in a retry, the arguments are
// the operation function error returned, and the upcoming sleep duration
func WithRetryFunc(retry RetryFunc) Option {
	return func(b *backoff) {
		b.retryFunc = retry
	}
}

// WithSleepFunc replaces the sleep function, this is internally used for unit tests
func WithSleepFunc(sleep SleepFunc) Option {
	return func(b *backoff) {
		b.sleepFunc = sleep
	}
}

// WithMinDuration set duration of first sleep
func WithMinDuration(d time.Duration) Option {
	return func(b *backoff) {
		b.Min = d
		b.current = d
	}
}

// WithMaxDuration caps off how long the sleep duration can be
func WithMaxDuration(d time.Duration) Option {
	return func(b *backoff) {
		b.Max = d
	}
}

// WithMaxAttempts limits the number of retry attempts until finally giving up
func WithMaxAttempts(attempts int) Option {
	return func(b *backoff) {
		b.Attempts = attempts
	}
}

func (b *backoff) retry(err error) error {
	b.attempt += 1
	if b.Attempts > 0 && b.attempt >= b.Attempts {
		return ErrMaxAttemptsReached
	}

	if b.retryFunc != nil {
		b.retryFunc(err, b.current)
	}
	b.sleepFunc(b.current)
	if b.current < b.Max {
		if b.current*2 > b.Max {
			b.current = b.Max
		} else {
			b.current *= 2
		}
	}

	return nil
}

func newBackoff(options []Option) backoff {
	backoff := backoff{
		Min:       time.Second,
		Max:       time.Minute,
		sleepFunc: time.Sleep,
		current:   time.Second,
		attempt:   0,
	}

	for _, option := range options {
		option(&backoff)
	}

	return backoff
}

// Retry attempts to run operation until it no longer returns an error.
// It will exponentially increase the time between each attempt until it reaches max.
//
// By default, it will start with a 1-second delay, which will double every attempt until it caps off at 1 minute.
// It will retry infinitely unless the WithMaxAttempts option is set
//
// returns nil or ErrMaxAttemptsReached
func Retry(operation OperationFunc, options ...Option) error {
	backoff := newBackoff(options)
	for {
		err := operation()
		if err == nil {
			return nil
		}

		err = backoff.retry(err)
		if err != nil {
			return err
		}
	}
}
