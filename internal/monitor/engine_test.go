package monitor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
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

func (p *scriptedProber) Fetch() ([]Event, bool, error) {
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

	if code := e.Run(); code != 0 {
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

	if code := e.Run(); code != 0 {
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

	if code := e.Run(); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Fatalf("out = %q, want empty", out.String())
	}
}
