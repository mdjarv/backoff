package backoff

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type retryCall struct {
	err error
	d   time.Duration
}

type retryHelper struct {
	operationRetVal error
	operationCalls  []interface{}
	sleepCalls      []time.Duration
	retryCalls      []retryCall
}

func (h *retryHelper) Operation() OperationFunc {
	return func() error {
		h.operationCalls = append(h.operationCalls, false)
		return h.operationRetVal
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
		operationRetVal: nil,
		operationCalls:  []interface{}{},
		sleepCalls:      []time.Duration{},
		retryCalls:      []retryCall{},
	}
}

func TestRetry(t *testing.T) {
	t.Run("immediate success should not delay or retry", func(t *testing.T) {
		helper := newRetryHelper()

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()))

		assert.NoError(t, err)
		assert.Lenf(t, helper.operationCalls, 1, "operation calls")
		assert.Empty(t, helper.retryCalls)
	})

	t.Run("max attempts", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxAttempts(10))

		assert.EqualError(t, err, "max attempts reached")
		assert.Lenf(t, helper.operationCalls, 10, "operation calls")
		assert.Lenf(t, helper.sleepCalls, 9, "sleep calls")
	})

	t.Run("on retry function", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxAttempts(3), WithRetryFunc(helper.Retry()))

		assert.EqualError(t, err, "max attempts reached")
		assert.Lenf(t, helper.operationCalls, 3, "operation calls")
		assert.Lenf(t, helper.sleepCalls, 2, "sleep calls")

		require.Lenf(t, helper.retryCalls, 2, "retry calls")
		assert.EqualErrorf(t, helper.retryCalls[0].err, "failed successfully", "first retry error")
		assert.Equalf(t, time.Second, helper.retryCalls[0].d, "first retry sleep duration")
		assert.EqualErrorf(t, helper.retryCalls[1].err, "failed successfully", "second retry error")
		assert.Equalf(t, 2*time.Second, helper.retryCalls[1].d, "second retry sleep duration")
	})

	t.Run("max duration", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMaxDuration(10*time.Second), WithMaxAttempts(7))
		assert.EqualError(t, err, "max attempts reached")
		require.Lenf(t, helper.sleepCalls, 6, "sleep calls")
		assert.Equalf(t, 1*time.Second, helper.sleepCalls[0], "first sleep")
		assert.Equalf(t, 2*time.Second, helper.sleepCalls[1], "second sleep")
		assert.Equalf(t, 4*time.Second, helper.sleepCalls[2], "third sleep")
		assert.Equalf(t, 8*time.Second, helper.sleepCalls[3], "fourth sleep")
		assert.Equalf(t, 10*time.Second, helper.sleepCalls[4], "fifth sleep")
		assert.Equalf(t, 10*time.Second, helper.sleepCalls[5], "final sleep")
	})

	t.Run("min duration", func(t *testing.T) {
		helper := newRetryHelper()
		helper.operationRetVal = fmt.Errorf("failed successfully")

		err := Retry(helper.Operation(), WithSleepFunc(helper.Sleep()), WithMinDuration(10*time.Second), WithMaxAttempts(3))
		assert.EqualError(t, err, "max attempts reached")
		require.Lenf(t, helper.sleepCalls, 2, "sleep calls")
		assert.Equalf(t, 10*time.Second, helper.sleepCalls[0], "first sleep")
		assert.Equalf(t, 20*time.Second, helper.sleepCalls[1], "second sleep")
	})
}
