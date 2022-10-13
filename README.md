# backoff

[![GoDoc](https://img.shields.io/badge/pkg.go.dev-doc-blue)](https://pkg.go.dev/github.com/mdjarv/backoff)

Package backoff provides a function for retrying a function with exponential backoff until the
provided operation either returns nil, indicating success, or it hits an attempts limit.

## Examples

### BasicUsage

Retry attempts to run operation until it no longer returns an error.
It will exponentially increase the time between each attempt until it reaches max.

By default, it will start with a 1-second delay, which will double every attempt until it caps off at 1 minute.
It will retry infinitely unless the WithMaxAttempts option is set

returns nil or ErrMaxAttemptsReached

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

Retry attempts to run operation until it no longer returns an error.
It will exponentially increase the time between each attempt until it reaches max.

By default, it will start with a 1-second delay, which will double every attempt until it caps off at 1 minute.
It will retry infinitely unless the WithMaxAttempts option is set

returns nil or ErrMaxAttemptsReached

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
