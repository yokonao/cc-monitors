package prcheck

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

func check(name, bucket string, run int) Check {
	return Check{Name: name, Bucket: bucket, Link: "https://ci.test/" + name + "/" + strconv.Itoa(run)}
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
	c := NewChecker("123")
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
	c := NewChecker("123")

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
	c := NewChecker("123")

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
	c := NewChecker("123")

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
	c := NewChecker("123")

	events, _ := c.Tick([]Check{check("test", "fail", 1), check("lint", "cancel", 1)})
	got := names(events, "check_failed")
	want := []string{"test", "lint"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("check_failed names = %v, want %v", got, want)
	}
}

func TestWaitsThroughEmptyChecks(t *testing.T) {
	c := NewChecker("123")

	events, done := c.Tick(nil)
	if done || len(events) != 0 {
		t.Fatalf("empty checks: events = %+v done = %v", events, done)
	}

	events, done = c.Tick([]Check{check("test", "pass", 1)})
	if !done || !reflect.DeepEqual(eventNames(events), []string{"checks_passed"}) {
		t.Fatalf("events = %+v done = %v", events, done)
	}
}
