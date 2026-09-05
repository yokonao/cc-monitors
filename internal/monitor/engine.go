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

// Prober fetches the next batch of events for one poll. Any retry policy for
// transient fetch failures belongs to the Prober — the engine treats a non-nil
// error as final and stops.
type Prober interface {
	Fetch() (events []Event, done bool, err error)
}

// Engine runs fetch -> emit -> sleep, repeating until Fetch reports done
// (exit 0) or a fetch error (exit 1).
type Engine struct {
	Prober   Prober
	Interval time.Duration
	Out      io.Writer
	Err      io.Writer
}

func (e *Engine) Run() int {
	for {
		events, done, err := e.Prober.Fetch()
		if err != nil {
			return 1
		}

		for _, ev := range events {
			e.emit(ev)
		}
		if done {
			return 0
		}
		time.Sleep(e.Interval)
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
