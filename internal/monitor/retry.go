package monitor

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Retry calls fn up to max times, waiting between attempts and logging each
// failure to log, so a Prober sees at most one terminal error per poll.
func Retry[T any](ctx context.Context, max int, wait time.Duration, log io.Writer, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		v, err := fn(ctx)
		if err == nil {
			return v, nil
		}
		lastErr = err
		_, _ = fmt.Fprintf(log, "fetch failed (%d/%d): %v\n", attempt, max, err)
		if attempt < max {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	_, _ = fmt.Fprintf(log, "giving up after %d consecutive fetch failures\n", max)
	return zero, lastErr
}
