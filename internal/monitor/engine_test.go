package monitor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// scriptedProber replays a fixed sequence of Fetch results: an error, or a
// state to hand to a fixed Tick function.
type scriptedProber struct {
	responses []any // int (state) or error
	i         int
	tick      func(int) ([]Event, bool)
}

func (p *scriptedProber) Fetch() (int, error) {
	if p.i >= len(p.responses) {
		return 0, errors.New("script exhausted (run didn't terminate)")
	}
	r := p.responses[p.i]
	p.i++
	if err, ok := r.(error); ok {
		return 0, err
	}
	return r.(int), nil
}

func (p *scriptedProber) Tick(state int) ([]Event, bool) {
	return p.tick(state)
}

func TestEngineEmitsEventsAndStopsWhenDone(t *testing.T) {
	prober := &scriptedProber{
		responses: []any{1},
		tick: func(n int) ([]Event, bool) {
			return []Event{{Name: "checks_passed", Data: map[string]any{"total": n}}}, true
		},
	}
	var out, errOut bytes.Buffer
	e := &Engine[int]{Prober: prober, MaxFetchFailures: 5, Out: &out, Err: &errOut}

	if code := e.Run(); code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if got := strings.TrimSpace(out.String()); got != `{"data":{"total":1},"event":"checks_passed"}` {
		t.Fatalf("out = %q", got)
	}
}

func TestEngineGivesUpAfterMaxFetchFailures(t *testing.T) {
	prober := &scriptedProber{
		responses: []any{errors.New("gh boom"), errors.New("gh boom")},
		tick:      func(int) ([]Event, bool) { return nil, true },
	}
	var out, errOut bytes.Buffer
	e := &Engine[int]{Prober: prober, MaxFetchFailures: 2, Out: &out, Err: &errOut}

	if code := e.Run(); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "fetch failed (2/2): gh boom") {
		t.Fatalf("err = %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "giving up after 2 consecutive fetch failures") {
		t.Fatalf("err = %q", errOut.String())
	}
}

func TestEngineResetsFailureCountOnSuccess(t *testing.T) {
	prober := &scriptedProber{
		responses: []any{errors.New("boom"), errors.New("boom"), errors.New("boom"), errors.New("boom"), 1},
		tick:      func(int) ([]Event, bool) { return nil, true },
	}
	var out, errOut bytes.Buffer
	e := &Engine[int]{Prober: prober, MaxFetchFailures: 5, Out: &out, Err: &errOut}

	if code := e.Run(); code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if strings.Contains(errOut.String(), "giving up") {
		t.Fatalf("err = %q, should not give up", errOut.String())
	}
}
