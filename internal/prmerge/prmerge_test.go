package prmerge

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

func watcher(states ...string) *Watcher {
	return &Watcher{
		Log: io.Discard,
		fetch: func(context.Context) (string, error) {
			s := states[0]
			states = states[1:]
			return s, nil
		},
	}
}

func TestFetch(t *testing.T) {
	for _, tc := range []struct {
		state string
		want  []monitor.Event
		done  bool
	}{
		{"OPEN", nil, false},
		{"MERGED", []monitor.Event{PRClosedEvent{Merged: true}}, true},
		{"CLOSED", []monitor.Event{PRClosedEvent{Merged: false}}, true},
	} {
		events, done, err := watcher(tc.state).Fetch(context.Background())
		if err != nil || done != tc.done || !reflect.DeepEqual(events, tc.want) {
			t.Errorf("%s: events = %+v, done = %v, err = %v", tc.state, events, done, err)
		}
	}
}

func TestFetchGivesUpAfterMaxFailures(t *testing.T) {
	calls := 0
	w := &Watcher{
		MaxFetchFailures: 2,
		Log:              io.Discard,
		fetch: func(context.Context) (string, error) {
			calls++
			return "", errors.New("boom")
		},
	}
	if _, _, err := w.Fetch(context.Background()); err == nil || calls != 2 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}
}

func TestFetchStateSurfacesExecError(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := fetchState(context.Background(), "1"); err == nil {
		t.Fatal("want error")
	}
}
