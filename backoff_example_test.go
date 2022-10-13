package backoff_test

import (
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
	// retry failed: max attempts reached
}
