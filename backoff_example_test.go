package backoff_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/mdjarv/backoff"
	"time"
)

func ExampleRetry_basicUsage() {
	_ = backoff.Retry(func() error {
		fmt.Println("success")
		return nil
	})
	// Output: success
}

func ExampleRetry_fullExample() {
	err := backoff.Retry(
		func() error {
			return fmt.Errorf("failed successfully")
		},
		backoff.WithMinDuration(100*time.Millisecond),
		backoff.WithMaxDuration(time.Second),
		backoff.WithMaxAttempts(7),
		backoff.WithRetryFunc(func(err error, duration time.Duration) {
			fmt.Printf("operation failed, retrying in %d ms\n", duration.Milliseconds())
		}),
		backoff.WithSleepFunc(func(duration time.Duration) {
			// Skip sleep in tests
		}),
	)

	if err != nil {
		fmt.Printf("retry failed: %s", err.Error())
	}

	// Output:
	// operation failed, retrying in 100 ms
	// operation failed, retrying in 200 ms
	// operation failed, retrying in 400 ms
	// operation failed, retrying in 800 ms
	// operation failed, retrying in 1000 ms
	// operation failed, retrying in 1000 ms
	// retry failed: max attempts reached after 7 attempts: failed successfully
}

func ExampleRetry_withContext() {
	ctx, cancel := context.WithCancel(context.Background())

	attempt := 0
	err := backoff.Retry(
		func() error {
			attempt++
			fmt.Printf("attempt %d\n", attempt)
			if attempt == 2 {
				cancel()
			}
			return fmt.Errorf("not yet")
		},
		backoff.WithContext(ctx),
		backoff.WithMinDuration(time.Millisecond),
		backoff.WithMaxDuration(time.Millisecond),
	)

	fmt.Println(err)

	// Output:
	// attempt 1
	// attempt 2
	// context canceled
}

func ExampleRetry_errorWrapping() {
	targetErr := fmt.Errorf("connection refused")

	err := backoff.Retry(
		func() error {
			return targetErr
		},
		backoff.WithMaxAttempts(1),
		backoff.WithSleepFunc(func(time.Duration) {}),
	)

	fmt.Println(errors.Is(err, backoff.ErrMaxAttemptsReached))
	fmt.Println(errors.Is(err, targetErr))

	var maxErr *backoff.MaxAttemptsError
	if errors.As(err, &maxErr) {
		fmt.Printf("attempts: %d\n", maxErr.Attempts)
	}

	// Output:
	// true
	// true
	// attempts: 1
}
