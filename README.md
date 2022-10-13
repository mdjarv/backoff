# backoff

Package backoff provides a function for retrying a function with exponential backoff until the
provided operation either returns nil, indicating success, or it hits an attempts limit.

## Functions

### func [Retry](/backoff.go#L117)

`func Retry(operation OperationFunc, options ...Option) error`

Retry attempts to run operation until it no longer returns an error.
It will exponentially increase the time between each attempt until it reaches max.

By default, it will start with a 1-second delay, which will double every attempt until it caps off at 1 minute.
It will retry infinitely unless the WithMaxAttempts option is set

returns nil or ErrMaxAttemptsReached

### BasicUsage

```golang
package main

import (
	"fmt"
	"github.com/mdjarv/backoff"
)

func main() {
	_ = backoff.Retry(func() error {
		fmt.Println("success")
		return nil
	})

}

```

 Output:

```
success
```

### FullExample

```golang
package main

import (
	"fmt"
	"github.com/mdjarv/backoff"
	"time"
)

func main() {
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

}

```

 Output:

```
operation failed, retrying in 100 ms
operation failed, retrying in 200 ms
operation failed, retrying in 400 ms
operation failed, retrying in 800 ms
operation failed, retrying in 1000 ms
operation failed, retrying in 1000 ms
retry failed: max attempts reached
```

## Types

### type [OperationFunc](/backoff.go#L15)

`type OperationFunc = func() error`

OperationFunc should return nil on success, otherwise returns an error

### type [Option](/backoff.go#L34)

`type Option func(*backoff)`

#### func [WithMaxAttempts](/backoff.go#L67)

`func WithMaxAttempts(attempts int) Option`

WithMaxAttempts limits the number of retry attempts until finally giving up

#### func [WithMaxDuration](/backoff.go#L60)

`func WithMaxDuration(d time.Duration) Option`

WithMaxDuration caps off how long the sleep duration can be

#### func [WithMinDuration](/backoff.go#L52)

`func WithMinDuration(d time.Duration) Option`

WithMinDuration set duration of first sleep

#### func [WithRetryFunc](/backoff.go#L38)

`func WithRetryFunc(retry RetryFunc) Option`

WithRetryFunc option is used to set a function to be executed before sleeping in a retry, the arguments are
the operation function error returned, and the upcoming sleep duration

#### func [WithSleepFunc](/backoff.go#L45)

`func WithSleepFunc(sleep SleepFunc) Option`

WithSleepFunc replaces the sleep function, this is internally used for unit tests

### type [RetryFunc](/backoff.go#L18)

`type RetryFunc = func(error, time.Duration)`

RetryFunc is executed on a failed operation execution, just before sleeping

### type [SleepFunc](/backoff.go#L21)

`type SleepFunc = func(time.Duration)`

SleepFunc can be used to replace the default time.Sleep function, for example in unit tests
