package prcheck

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

func check(name, bucket string, run int) Check {
	return Check{Name: name, Bucket: bucket, Link: "https://ci.test/" + name + "/" + strconv.Itoa(run)}
}

func TestFetchChecksSurfacesExecErrorWhenGHMissing(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := FetchChecks(context.Background(), "123")
	if err == nil {
		t.Fatalf("want error")
	}
	if err.Error() == "no output from gh pr checks" {
		t.Fatalf("err should surface the real exec failure, not the generic fallback: %v", err)
	}
	if !strings.Contains(err.Error(), "gh") {
		t.Fatalf("err = %v, want it to mention gh", err)
	}
}

func names(events []monitor.Event, name string) []string {
	var out []string
	for _, e := range events {
		if e.Name == name {
			out = append(out, e.Data["name"].(string))
		}
	}
	return out
}

func eventNames(events []monitor.Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Name)
	}
	return out
}

func TestAllGreenPasses(t *testing.T) {
	c := NewChecker("123", 0)
	events, done := c.Tick([]Check{check("test", "pass", 1), check("lint", "skipping", 1)})
	if !done {
		t.Fatalf("want done, got not done")
	}
	want := []monitor.Event{{Name: "checks_passed", Data: map[string]any{"total": 2}}}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %+v, want %+v", events, want)
	}
}

func TestFixLoopReportsFailureThenPasses(t *testing.T) {
	c := NewChecker("123", 0)

	events, done := c.Tick([]Check{check("test", "fail", 1)})
	if done {
		t.Fatalf("want not done after failure")
	}
	if got := eventNames(events); !reflect.DeepEqual(got, []string{"check_failed"}) {
		t.Fatalf("events = %v", got)
	}
	if events[0].Data["name"] != "test" || events[0].Data["url"] != "https://ci.test/test/1" {
		t.Fatalf("data = %+v", events[0].Data)
	}

	events, done = c.Tick([]Check{check("test", "pass", 1)})
	if !done {
		t.Fatalf("want done after fix")
	}
	if got := eventNames(events); !reflect.DeepEqual(got, []string{"checks_passed"}) {
		t.Fatalf("events = %v", got)
	}
}

func TestFailureEmittedOnceWhileStillRed(t *testing.T) {
	c := NewChecker("123", 0)

	events, _ := c.Tick([]Check{check("test", "fail", 1)})
	if len(events) != 1 {
		t.Fatalf("first tick events = %+v", events)
	}

	events, done := c.Tick([]Check{check("test", "fail", 1)})
	if len(events) != 0 {
		t.Fatalf("re-poll of same failure should emit nothing, got %+v", events)
	}
	if done {
		t.Fatalf("want not done while still red")
	}

	events, done = c.Tick([]Check{check("test", "pass", 1)})
	if !done || !reflect.DeepEqual(eventNames(events), []string{"checks_passed"}) {
		t.Fatalf("events = %+v done = %v", events, done)
	}
}

// A fix push starts a new run (new link); it must re-report even when the
// intervening pending/green state fell between polls and was never observed.
func TestNewRunReReportsFailure(t *testing.T) {
	c := NewChecker("123", 0)

	events, _ := c.Tick([]Check{check("test", "fail", 1)})
	if len(names(events, "check_failed")) != 1 {
		t.Fatalf("run 1 events = %+v", events)
	}

	events, _ = c.Tick([]Check{check("test", "fail", 2)})
	if len(names(events, "check_failed")) != 1 {
		t.Fatalf("run 2 should re-report, got %+v", events)
	}

	events, done := c.Tick([]Check{check("test", "pass", 2)})
	if !done || !reflect.DeepEqual(eventNames(events), []string{"checks_passed"}) {
		t.Fatalf("events = %+v done = %v", events, done)
	}
}

func TestEachRedCheckReported(t *testing.T) {
	c := NewChecker("123", 0)

	events, _ := c.Tick([]Check{check("test", "fail", 1), check("lint", "cancel", 1)})
	got := names(events, "check_failed")
	want := []string{"test", "lint"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("check_failed names = %v, want %v", got, want)
	}
}

func TestWaitsThroughEmptyChecks(t *testing.T) {
	c := NewChecker("123", 0)

	events, done := c.Tick(nil)
	if done || len(events) != 0 {
		t.Fatalf("empty checks: events = %+v done = %v", events, done)
	}

	events, done = c.Tick([]Check{check("test", "pass", 1)})
	if !done || !reflect.DeepEqual(eventNames(events), []string{"checks_passed"}) {
		t.Fatalf("events = %+v done = %v", events, done)
	}
}

func TestFetchRetriesThenSucceeds(t *testing.T) {
	c := NewChecker("123", 0)
	c.MaxFetchFailures = 3
	var log bytes.Buffer
	c.Log = &log

	calls := 0
	c.fetch = func(context.Context, string) ([]Check, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("boom")
		}
		return []Check{check("test", "pass", 1)}, nil
	}

	events, done, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !done || !reflect.DeepEqual(eventNames(events), []string{"checks_passed"}) {
		t.Fatalf("events = %+v done = %v", events, done)
	}
	if !strings.Contains(log.String(), "fetch failed (1/3): boom") || !strings.Contains(log.String(), "fetch failed (2/3): boom") {
		t.Fatalf("log = %q", log.String())
	}
	if strings.Contains(log.String(), "giving up") {
		t.Fatalf("should not give up: %q", log.String())
	}
}

func TestFetchGivesUpAfterMaxFailures(t *testing.T) {
	c := NewChecker("123", 0)
	c.MaxFetchFailures = 2
	var log bytes.Buffer
	c.Log = &log
	c.fetch = func(context.Context, string) ([]Check, error) { return nil, errors.New("gh boom") }

	_, done, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatalf("want error")
	}
	if done {
		t.Fatalf("want not done")
	}
	if !strings.Contains(log.String(), "fetch failed (2/2): gh boom") {
		t.Fatalf("log = %q", log.String())
	}
	if !strings.Contains(log.String(), "giving up after 2 consecutive fetch failures") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestFetchStopsPromptlyWhenContextCanceled(t *testing.T) {
	c := NewChecker("123", time.Hour)
	c.MaxFetchFailures = 5
	var log bytes.Buffer
	c.Log = &log

	ctx, cancel := context.WithCancel(context.Background())
	c.fetch = func(context.Context, string) ([]Check, error) {
		cancel()
		return nil, errors.New("boom")
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := c.Fetch(ctx)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("want error")
		}
	case <-time.After(time.Second):
		t.Fatal("Fetch did not return promptly after context cancellation")
	}
}
