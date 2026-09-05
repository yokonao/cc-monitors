package monitor

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fetchResult struct {
	events []Event
	done   bool
	err    error
}

// scriptedProber replays a fixed sequence of Fetch results.
type scriptedProber struct {
	responses []fetchResult
	i         int
}

func (p *scriptedProber) Fetch(context.Context) ([]Event, bool, error) {
	if p.i >= len(p.responses) {
		return nil, false, errors.New("script exhausted (run didn't terminate)")
	}
	r := p.responses[p.i]
	p.i++
	return r.events, r.done, r.err
}

func TestEngineEmitsEventsAndStopsWhenDone(t *testing.T) {
	prober := &scriptedProber{responses: []fetchResult{
		{events: []Event{{Name: "checks_passed", Data: map[string]any{"total": 1}}}, done: true},
	}}
	var out, errOut bytes.Buffer
	e := &Engine{Prober: prober, Out: &out, Err: &errOut}

	if code := e.Run(context.Background()); code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if got := strings.TrimSpace(out.String()); got != `{"data":{"total":1},"event":"checks_passed"}` {
		t.Fatalf("out = %q", got)
	}
}

func TestEngineKeepsPollingUntilDone(t *testing.T) {
	prober := &scriptedProber{responses: []fetchResult{
		{done: false},
		{events: []Event{{Name: "checks_passed", Data: map[string]any{"total": 1}}}, done: true},
	}}
	var out, errOut bytes.Buffer
	e := &Engine{Prober: prober, Out: &out, Err: &errOut}

	if code := e.Run(context.Background()); code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if p := prober.i; p != 2 {
		t.Fatalf("Fetch called %d times, want 2", p)
	}
}

func TestEngineStopsOnFetchError(t *testing.T) {
	prober := &scriptedProber{responses: []fetchResult{{err: errors.New("gh boom")}}}
	var out, errOut bytes.Buffer
	e := &Engine{Prober: prober, Out: &out, Err: &errOut}

	if code := e.Run(context.Background()); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Fatalf("out = %q, want empty", out.String())
	}
}

func TestEngineStopsPromptlyWhenContextCanceled(t *testing.T) {
	prober := &scriptedProber{responses: []fetchResult{{done: false}}}
	var out, errOut bytes.Buffer
	e := &Engine{Prober: prober, Interval: time.Hour, Out: &out, Err: &errOut}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan int, 1)
	go func() { done <- e.Run(ctx) }()

	select {
	case code := <-done:
		if code != 1 {
			t.Fatalf("code = %d, want 1", code)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return promptly after context cancellation")
	}
}
