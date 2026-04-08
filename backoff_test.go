package backoff

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retryCall struct {
	err error
	d   time.Duration
}

type retryHelper struct {
	operationRetVal error
	operationCalls  int
	sleepCalls      []time.Duration
	retryCalls      []retryCall
}

func (h *retryHelper) Operation() OperationFunc {
	return func() error {
		h.operationCalls++
		return h.operationRetVal
	}
}

func (h *retryHelper) OperationFailN(n int) OperationFunc {
	calls := 0
	return func() error {
		h.operationCalls++
		calls++
		if calls <= n {
			return h.operationRetVal
		}
		return nil
	}
}

func (h *retryHelper) Sleep() SleepFunc {
	return func(d time.Duration) {
		h.sleepCalls = append(h.sleepCalls, d)
	}
}

func (h *retryHelper) Retry() RetryFunc {
	return func(err error, d time.Duration) {
		h.retryCalls = append(h.retryCalls, retryCall{err: err, d: d})
	}
}

func newRetryHelper() retryHelper {
	return retryHelper{
		sleepCalls: []time.Duration{},
		retryCalls: []retryCall{},
	}
}

func TestRetry(t *testing.T) {
	t.Run("immediate success should not delay or retry", func(t *testing.T) {
		helper := newRetryHelper()

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()))

		assert.NoError(t, err)
		assert.Equal(t, 1, helper.operationCalls, "operation calls")
		assert.Empty(t, helper.retryCalls)
	})

	t.Run("max attempts", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxAttempts(10))

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrMaxAttemptsReached))
		assert.Equal(t, 10, helper.operationCalls, "operation calls")
		assert.Len(t, helper.sleepCalls, 9, "sleep calls")

		var maxErr *MaxAttemptsError
		require.True(t, errors.As(err, &maxErr))
		assert.Equal(t, 10, maxErr.Attempts)
		assert.EqualError(t, maxErr.Last, "failed successfully")
	})

	t.Run("on retry function", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxAttempts(3), WithRetryFunc(helper.Retry()))

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrMaxAttemptsReached))
		assert.Equal(t, 3, helper.operationCalls, "operation calls")
		assert.Len(t, helper.sleepCalls, 2, "sleep calls")

		require.Len(t, helper.retryCalls, 2, "retry calls")
		assert.EqualError(t, helper.retryCalls[0].err, "failed successfully", "first retry error")
		assert.Equal(t, time.Second, helper.retryCalls[0].d, "first retry sleep duration")
		assert.EqualError(t, helper.retryCalls[1].err, "failed successfully", "second retry error")
		assert.Equal(t, 2*time.Second, helper.retryCalls[1].d, "second retry sleep duration")
	})

	t.Run("max duration", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxDuration(10*time.Second), WithMaxAttempts(7))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrMaxAttemptsReached))
		require.Len(t, helper.sleepCalls, 6, "sleep calls")
		assert.Equal(t, 1*time.Second, helper.sleepCalls[0], "first sleep")
		assert.Equal(t, 2*time.Second, helper.sleepCalls[1], "second sleep")
		assert.Equal(t, 4*time.Second, helper.sleepCalls[2], "third sleep")
		assert.Equal(t, 8*time.Second, helper.sleepCalls[3], "fourth sleep")
		assert.Equal(t, 10*time.Second, helper.sleepCalls[4], "fifth sleep")
		assert.Equal(t, 10*time.Second, helper.sleepCalls[5], "final sleep")
	})

	t.Run("min duration", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMinDuration(10*time.Second), WithMaxAttempts(3))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrMaxAttemptsReached))
		require.Len(t, helper.sleepCalls, 2, "sleep calls")
		assert.Equal(t, 10*time.Second, helper.sleepCalls[0], "first sleep")
		assert.Equal(t, 20*time.Second, helper.sleepCalls[1], "second sleep")
	})

	t.Run("succeeds after failures", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("temporary failure")

		err := Retry(helper.OperationFailN(3), WithSleepFunc(helper.Sleep()), WithRetryFunc(helper.Retry()))

		assert.NoError(t, err)
		assert.Equal(t, 4, helper.operationCalls, "operation calls")
		assert.Len(t, helper.sleepCalls, 3, "sleep calls")
		assert.Len(t, helper.retryCalls, 3, "retry calls")
	})
}

func TestMaxAttemptsError(t *testing.T) {
	t.Run("matches sentinel via errors.Is", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("underlying")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxAttempts(1))

		assert.True(t, errors.Is(err, ErrMaxAttemptsReached))
	})

	t.Run("unwraps to last operation error", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("underlying cause")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxAttempts(1))

		var target *MaxAttemptsError
		require.True(t, errors.As(err, &target))
		assert.Equal(t, 1, target.Attempts)
		assert.EqualError(t, target.Last, "underlying cause")
		assert.EqualError(t, errors.Unwrap(err), "underlying cause")
	})

	t.Run("error message includes details", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("connection refused")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxAttempts(3))

		assert.EqualError(t, err, "max attempts reached after 3 attempts: connection refused")
	})
}

func TestRetryContext(t *testing.T) {
	t.Run("cancelled before first attempt", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		calls := 0
		err := Retry(func() error {
			calls++
			return fmt.Errorf("should not reach")
		}, WithContext(ctx))

		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, calls)
	})

	t.Run("cancelled during sleep", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		calls := 0
		start := time.Now()
		err := Retry(func() error {
			calls++
			return fmt.Errorf("keep failing")
		}, WithContext(ctx), WithMinDuration(10*time.Second))

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, 1, calls)
		assert.Less(t, time.Since(start), time.Second, "should not have slept full duration")
	})

	t.Run("context sleep completes normally", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		calls := 0
		err := Retry(func() error {
			calls++
			if calls < 3 {
				return fmt.Errorf("not yet")
			}
			return nil
		}, WithContext(ctx), WithMinDuration(time.Millisecond))

		assert.NoError(t, err)
		assert.Equal(t, 3, calls)
	})

	t.Run("succeeds before cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := Retry(func() error {
			return nil
		}, WithContext(ctx))

		assert.NoError(t, err)
	})
}

func TestRetryJitter(t *testing.T) {
	t.Run("jitter produces varying durations", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("fail")

		err := Retry(helper.Operation(),
			WithSleepFunc(helper.Sleep()),
			WithMaxAttempts(20),
			WithJitter(true),
		)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrMaxAttemptsReached))

		// With 19 sleeps and jitter, at least some durations should differ
		unique := make(map[time.Duration]bool)
		for _, d := range helper.sleepCalls {
			unique[d] = true
		}
		assert.Greater(t, len(unique), 1, "jitter should produce varying durations")
	})

	t.Run("jitter durations are bounded", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("fail")

		maxDur := 10 * time.Second
		err := Retry(helper.Operation(),
			WithSleepFunc(helper.Sleep()),
			WithMaxAttempts(50),
			WithMaxDuration(maxDur),
			WithJitter(true),
		)

		require.Error(t, err)
		for i, d := range helper.sleepCalls {
			assert.GreaterOrEqual(t, d, time.Duration(0), "sleep %d must be non-negative", i)
			assert.Less(t, d, maxDur, "sleep %d must be less than max duration", i)
		}
	})

	t.Run("retryFunc receives jittered duration", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("fail")

		err := Retry(helper.Operation(),
			WithSleepFunc(helper.Sleep()),
			WithMaxAttempts(5),
			WithRetryFunc(helper.Retry()),
			WithJitter(true),
		)

		require.Error(t, err)
		// retryFunc and sleep should receive the same durations
		require.Len(t, helper.retryCalls, len(helper.sleepCalls))
		for i := range helper.retryCalls {
			assert.Equal(t, helper.retryCalls[i].d, helper.sleepCalls[i],
				"retryFunc and sleep should get same duration at index %d", i)
		}
	})
}

func TestRetryValidation(t *testing.T) {
	noop := func() error { return nil }

	t.Run("negative min duration", func(t *testing.T) {
		err := Retry(noop, WithMinDuration(-time.Second))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MinDuration must be positive")
	})

	t.Run("zero min duration", func(t *testing.T) {
		err := Retry(noop, WithMinDuration(0))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MinDuration must be positive")
	})

	t.Run("negative max duration", func(t *testing.T) {
		err := Retry(noop, WithMaxDuration(-time.Second))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxDuration must be positive")
	})

	t.Run("zero max duration", func(t *testing.T) {
		err := Retry(noop, WithMaxDuration(0))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxDuration must be positive")
	})

	t.Run("min exceeds max", func(t *testing.T) {
		err := Retry(noop, WithMinDuration(time.Minute), WithMaxDuration(time.Second))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not exceed")
	})

	t.Run("negative attempts", func(t *testing.T) {
		err := Retry(noop, WithMaxAttempts(-1))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxAttempts must be non-negative")
	})

	t.Run("zero attempts means infinite", func(t *testing.T) {
		helper := newRetryHelper()

		calls := 0
		err := Retry(func() error {
			calls++
			if calls >= 5 {
				return nil
			}
			return fmt.Errorf("fail")
		}, WithSleepFunc(helper.Sleep()), WithMaxAttempts(0))

		assert.NoError(t, err)
		assert.Equal(t, 5, calls)
	})
}
