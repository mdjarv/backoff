/*
Package backoff provides exponential backoff with optional jitter for retrying
operations until success or a terminal condition (max attempts, context cancellation).
*/
package backoff

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// ErrMaxAttemptsReached is a sentinel for use with errors.Is.
// The actual returned error is *MaxAttemptsError which wraps the last operation error.
var ErrMaxAttemptsReached = errors.New("max attempts reached")

// MaxAttemptsError is returned when the retry limit is exhausted.
// It matches ErrMaxAttemptsReached via errors.Is and wraps the last operation error.
type MaxAttemptsError struct {
	Attempts int
	Last     error
}

func (e *MaxAttemptsError) Error() string {
	return fmt.Sprintf("max attempts reached after %d attempts: %s", e.Attempts, e.Last)
}

func (e *MaxAttemptsError) Is(target error) bool {
	return target == ErrMaxAttemptsReached
}

func (e *MaxAttemptsError) Unwrap() error {
	return e.Last
}

// OperationFunc should return nil on success, otherwise returns an error.
type OperationFunc = func() error

// RetryFunc is called on a failed attempt, just before sleeping.
// Arguments are the operation error and the upcoming sleep duration (after jitter, if enabled).
type RetryFunc = func(error, time.Duration)

// SleepFunc can replace time.Sleep, for example in unit tests.
// Ignored when WithContext is set (context-aware sleep is used instead).
type SleepFunc = func(time.Duration)

// Option configures the backoff behavior.
type Option func(*backoff)

// WithRetryFunc sets a callback executed before each retry sleep.
func WithRetryFunc(retry RetryFunc) Option {
	return func(b *backoff) {
		b.retryFunc = retry
	}
}

// WithSleepFunc replaces the sleep function. Useful for unit tests.
// Ignored when WithContext is also set.
func WithSleepFunc(sleep SleepFunc) Option {
	return func(b *backoff) {
		b.sleepFunc = sleep
	}
}

// WithMinDuration sets the initial sleep duration. Must be positive.
func WithMinDuration(d time.Duration) Option {
	return func(b *backoff) {
		b.Min = d
		b.current = d
	}
}

// WithMaxDuration caps the sleep duration. Must be positive and >= MinDuration.
func WithMaxDuration(d time.Duration) Option {
	return func(b *backoff) {
		b.Max = d
	}
}

// WithMaxAttempts limits total attempts. Must be positive.
// When exhausted, Retry returns *MaxAttemptsError (matches ErrMaxAttemptsReached via errors.Is).
func WithMaxAttempts(attempts int) Option {
	return func(b *backoff) {
		b.Attempts = attempts
	}
}

// WithContext sets a context for cancellation support.
// Retry checks context before each attempt and uses context-aware sleep
// (overriding any SleepFunc).
func WithContext(ctx context.Context) Option {
	return func(b *backoff) {
		b.ctx = ctx
	}
}

// WithJitter enables full jitter on sleep durations.
// Actual sleep is uniformly distributed in [0, calculated_duration).
// Prevents thundering herd when multiple clients retry against the same service.
func WithJitter(enabled bool) Option {
	return func(b *backoff) {
		b.jitter = enabled
	}
}

type backoff struct {
	Min      time.Duration
	Max      time.Duration
	Attempts int

	retryFunc RetryFunc
	sleepFunc SleepFunc
	ctx       context.Context
	jitter    bool
	rng       *rand.Rand
	current   time.Duration
	attempt   int
}

func (b *backoff) sleepDuration() time.Duration {
	d := b.current
	if b.jitter && d > 0 {
		d = time.Duration(b.rng.Int63n(int64(d)))
	}
	return d
}

func (b *backoff) sleep(d time.Duration) error {
	if b.ctx != nil {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-b.ctx.Done():
			return b.ctx.Err()
		}
	}
	b.sleepFunc(d)
	return nil
}

func (b *backoff) retry(err error) error {
	b.attempt++
	if b.Attempts > 0 && b.attempt >= b.Attempts {
		return &MaxAttemptsError{Attempts: b.attempt, Last: err}
	}

	sleepDur := b.sleepDuration()

	if b.retryFunc != nil {
		b.retryFunc(err, sleepDur)
	}

	if sleepErr := b.sleep(sleepDur); sleepErr != nil {
		return sleepErr
	}

	if b.current < b.Max {
		b.current *= 2
		if b.current > b.Max {
			b.current = b.Max
		}
	}

	return nil
}

func validate(b *backoff) error {
	if b.Min <= 0 {
		return fmt.Errorf("backoff: MinDuration must be positive, got %s", b.Min)
	}
	if b.Max <= 0 {
		return fmt.Errorf("backoff: MaxDuration must be positive, got %s", b.Max)
	}
	if b.Min > b.Max {
		return fmt.Errorf("backoff: MinDuration (%s) must not exceed MaxDuration (%s)", b.Min, b.Max)
	}
	if b.Attempts < 0 {
		return fmt.Errorf("backoff: MaxAttempts must be non-negative, got %d", b.Attempts)
	}
	return nil
}

func newBackoff(options []Option) (*backoff, error) {
	b := &backoff{
		Min:       time.Second,
		Max:       time.Minute,
		sleepFunc: time.Sleep,
		current:   time.Second,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for _, option := range options {
		option(b)
	}

	if err := validate(b); err != nil {
		return nil, err
	}

	return b, nil
}

// Retry runs operation until it returns nil, max attempts are exhausted,
// or the context is cancelled.
//
// By default, starts with a 1-second delay, doubling each attempt up to 1 minute.
// Retries infinitely unless WithMaxAttempts is set.
//
// Returns nil on success, *MaxAttemptsError when attempts are exhausted (matches
// ErrMaxAttemptsReached via errors.Is), context error on cancellation, or a
// validation error for invalid options.
func Retry(operation OperationFunc, options ...Option) error {
	b, err := newBackoff(options)
	if err != nil {
		return err
	}

	for {
		if b.ctx != nil {
			if err := b.ctx.Err(); err != nil {
				return err
			}
		}

		err := operation()
		if err == nil {
			return nil
		}

		if retryErr := b.retry(err); retryErr != nil {
			return retryErr
		}
	}
}
