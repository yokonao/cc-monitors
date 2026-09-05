package monitor

import (
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

// Prober fetches state and turns it into events, deciding whether polling should
// stop. Tick is only called after a successful Fetch.
type Prober[T any] interface {
	Fetch() (T, error)
	Tick(T) (events []Event, done bool)
}

// Engine runs fetch -> tick -> emit -> sleep, repeating until Tick reports done
// (exit 0) or MaxFetchFailures consecutive Fetch errors occur (exit 1).
type Engine[T any] struct {
	Prober           Prober[T]
	Interval         time.Duration
	MaxFetchFailures int
	Out              io.Writer
	Err              io.Writer
}

func (e *Engine[T]) Run() int {
	failures := 0
	for {
		state, err := e.Prober.Fetch()
		if err != nil {
			failures++
			fmt.Fprintf(e.Err, "fetch failed (%d/%d): %v\n", failures, e.MaxFetchFailures, err)
			if failures >= e.MaxFetchFailures {
				fmt.Fprintf(e.Err, "giving up after %d consecutive fetch failures\n", e.MaxFetchFailures)
				return 1
			}
			time.Sleep(e.Interval)
			continue
		}
		failures = 0

		events, done := e.Prober.Tick(state)
		for _, ev := range events {
			e.emit(ev)
		}
		if done {
			return 0
		}
		time.Sleep(e.Interval)
	}
}

func (e *Engine[T]) emit(ev Event) {
	line, err := json.Marshal(map[string]any{"event": ev.Name, "data": ev.Data})
	if err != nil {
		fmt.Fprintf(e.Err, "encode event %q: %v\n", ev.Name, err)
		return
	}
	fmt.Fprintln(e.Out, string(line))
}
