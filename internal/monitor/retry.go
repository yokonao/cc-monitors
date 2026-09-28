package monitor

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Retry calls fn at most attempts times, waiting between attempts and logging each
// failure to log, so a Prober sees at most one terminal error per poll.
func Retry[T any](ctx context.Context, attempts int, wait time.Duration, log io.Writer, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		v, err := fn(ctx)
		if err == nil {
			return v, nil
		}
		lastErr = err
		_, _ = fmt.Fprintf(log, "fetch failed (%d/%d): %v\n", attempt, attempts, err)
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	_, _ = fmt.Fprintf(log, "giving up after %d consecutive fetch failures\n", attempts)
	return zero, lastErr
}
