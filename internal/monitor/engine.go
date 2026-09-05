package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Event is a line-delimited JSON event emitted as {"event": Name, "data": Data}.
type Event struct {
	Name string
	Data map[string]any
}

// Prober fetches the next batch of events for one poll. Any retry policy for
// transient fetch failures belongs to the Prober — the engine treats a non-nil
// error as final and stops. Fetch should return promptly once ctx is done.
type Prober interface {
	Fetch(ctx context.Context) (events []Event, done bool, err error)
}

// Engine runs fetch -> emit -> sleep, repeating until Fetch reports done
// (exit 0), a fetch error (exit 1), or ctx is canceled (exit 1).
type Engine struct {
	Prober   Prober
	Interval time.Duration
	Out      io.Writer
	Err      io.Writer
}

// Run returns the process exit code, plus the error that caused a non-zero
// exit (nil when the run finished cleanly).
func (e *Engine) Run(ctx context.Context) (int, error) {
	for {
		events, done, err := e.Prober.Fetch(ctx)
		if err != nil {
			return 1, err
		}

		for _, ev := range events {
			e.emit(ev)
		}
		if done {
			return 0, nil
		}

		select {
		case <-ctx.Done():
			return 1, ctx.Err()
		case <-time.After(e.Interval):
		}
	}
}

func (e *Engine) emit(ev Event) {
	line, err := json.Marshal(map[string]any{"event": ev.Name, "data": ev.Data})
	if err != nil {
		fmt.Fprintf(e.Err, "encode event %q: %v\n", ev.Name, err)
		return
	}
	fmt.Fprintln(e.Out, string(line))
}
