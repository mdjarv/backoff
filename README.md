# backoff

[![GoDoc](https://img.shields.io/badge/pkg.go.dev-doc-blue)](https://pkg.go.dev/github.com/mdjarv/backoff)

Package backoff provides exponential backoff with optional jitter for retrying
operations until success or a terminal condition (max attempts, context cancellation).

## Features

- Exponential backoff with configurable min/max durations
- Context cancellation support
- Full jitter to prevent thundering herd
- Rich error wrapping — unwrap to get the last operation error
- Input validation for all options

## Install

```
go get github.com/mdjarv/backoff
```

## Usage

### Basic

```go
err := backoff.Retry(func() error {
    return doSomething()
})
```

### Full Example

```go
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
```

Output:

```
operation failed, retrying in 100 ms
operation failed, retrying in 200 ms
operation failed, retrying in 400 ms
operation failed, retrying in 800 ms
operation failed, retrying in 1000 ms
operation failed, retrying in 1000 ms
retry failed: max attempts reached after 7 attempts: failed successfully
```

### With Context

Use `WithContext` to cancel retries when a context is done:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := backoff.Retry(
    func() error {
        return callExternalService()
    },
    backoff.WithContext(ctx),
    backoff.WithMaxDuration(5*time.Second),
)
```

When the context is cancelled, `Retry` returns the context error (`context.Canceled` or `context.DeadlineExceeded`).

### With Jitter

Enable jitter to randomize sleep durations and prevent thundering herd:

```go
err := backoff.Retry(
    func() error {
        return callSharedService()
    },
    backoff.WithJitter(true),
    backoff.WithMinDuration(500*time.Millisecond),
    backoff.WithMaxDuration(30*time.Second),
)
```

### Error Handling

When max attempts are exhausted, `Retry` returns a `*MaxAttemptsError` that:
- Matches `backoff.ErrMaxAttemptsReached` via `errors.Is`
- Wraps the last operation error (accessible via `errors.Unwrap` or `errors.As`)

```go
err := backoff.Retry(operation, backoff.WithMaxAttempts(5))

if errors.Is(err, backoff.ErrMaxAttemptsReached) {
    fmt.Println("gave up after max attempts")
}

// Unwrap to inspect the original error
var connErr *net.OpError
if errors.As(err, &connErr) {
    fmt.Println("last error was a network error:", connErr)
}

// Or access MaxAttemptsError directly
var maxErr *backoff.MaxAttemptsError
if errors.As(err, &maxErr) {
    fmt.Printf("failed after %d attempts: %v\n", maxErr.Attempts, maxErr.Last)
}
```

## Options

| Option | Default | Description |
|---|---|---|
| `WithMinDuration(d)` | 1s | Initial sleep duration |
| `WithMaxDuration(d)` | 1m | Maximum sleep duration cap |
| `WithMaxAttempts(n)` | 0 (infinite) | Maximum number of attempts |
| `WithContext(ctx)` | nil | Context for cancellation |
| `WithJitter(bool)` | false | Randomize sleep in `[0, duration)` |
| `WithRetryFunc(fn)` | nil | Callback before each retry sleep |
| `WithSleepFunc(fn)` | `time.Sleep` | Custom sleep (ignored with `WithContext`) |

## License

[MIT](LICENSE.md)
