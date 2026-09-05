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

	if code, err := e.Run(context.Background()); code != 0 || err != nil {
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

	if code, err := e.Run(context.Background()); code != 0 || err != nil {
		t.Fatalf("code = %d, want 0", code)
	}
	if p := prober.i; p != 2 {
		t.Fatalf("Fetch called %d times, want 2", p)
	}
}

func TestEngineStopsOnFetchError(t *testing.T) {
	wantErr := errors.New("gh boom")
	prober := &scriptedProber{responses: []fetchResult{{err: wantErr}}}
	var out, errOut bytes.Buffer
	e := &Engine{Prober: prober, Out: &out, Err: &errOut}

	code, err := e.Run(context.Background())
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if out.Len() != 0 {
		t.Fatalf("out = %q, want empty", out.String())
	}
}

// hangingProber simulates a Prober that blocks until ctx says stop — the
// shape of a real subprocess call wired to exec.CommandContext — to check
// that Engine.FetchTimeout bounds it even though the Prober never gives up
// on its own.
type hangingProber struct{}

func (hangingProber) Fetch(ctx context.Context) ([]Event, bool, error) {
	<-ctx.Done()
	return nil, false, ctx.Err()
}

func TestEngineFetchTimeoutBoundsHangingProber(t *testing.T) {
	var out, errOut bytes.Buffer
	e := &Engine{Prober: hangingProber{}, FetchTimeout: 10 * time.Millisecond, Out: &out, Err: &errOut}

	type result struct {
		code int
		err  error
	}
	done := make(chan result, 1)
	go func() {
		code, err := e.Run(context.Background())
		done <- result{code, err}
	}()

	select {
	case r := <-done:
		if r.code != 1 {
			t.Fatalf("code = %d, want 1", r.code)
		}
		if !errors.Is(r.err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return within FetchTimeout")
	}
}

func TestEngineStopsPromptlyWhenContextCanceled(t *testing.T) {
	prober := &scriptedProber{responses: []fetchResult{{done: false}}}
	var out, errOut bytes.Buffer
	e := &Engine{Prober: prober, Interval: time.Hour, Out: &out, Err: &errOut}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	type result struct {
		code int
		err  error
	}
	done := make(chan result, 1)
	go func() {
		code, err := e.Run(ctx)
		done <- result{code, err}
	}()

	select {
	case r := <-done:
		if r.code != 1 {
			t.Fatalf("code = %d, want 1", r.code)
		}
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return promptly after context cancellation")
	}
}
