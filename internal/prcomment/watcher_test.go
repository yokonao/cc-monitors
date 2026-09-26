package prcomment

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func newComment(kind string, id int64, author string, min int) comment {
	return comment{Kind: kind, ID: id, Author: author, Body: "hi", API: fmt.Sprintf("https://api.test/%s/%d", kind, id), CreatedAt: t0.Add(time.Duration(min) * time.Minute)}
}

func newWatcher() *Watcher {
	w := NewWatcher("123", 0)
	w.self = "me"
	return w
}

func eventNames(events []monitor.Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.EventName())
	}
	return out
}

func TestBaselineIsSilent(t *testing.T) {
	w := newWatcher()
	events, done := w.tick(snapshot{State: "OPEN", Comments: []comment{newComment("comment", 1, "alice", 0)}})
	if done || len(events) != 0 {
		t.Fatalf("events = %+v done = %v", events, done)
	}

	events, _ = w.tick(snapshot{State: "OPEN", Comments: []comment{newComment("comment", 1, "alice", 0)}})
	if len(events) != 0 {
		t.Fatalf("re-poll should emit nothing, got %+v", events)
	}
}

func TestNewCommentsEmittedInOrder(t *testing.T) {
	w := newWatcher()
	w.tick(snapshot{State: "OPEN"})

	events, done := w.tick(snapshot{State: "OPEN", Comments: []comment{
		newComment("review_comment", 1, "alice", 2),
		newComment("comment", 1, "bob", 1),
	}})
	if done {
		t.Fatalf("want not done")
	}
	want := []monitor.Event{
		NewCommentEvent{Kind: "comment", ID: 1, API: "https://api.test/comment/1"},
		NewCommentEvent{Kind: "review_comment", ID: 1, API: "https://api.test/review_comment/1"},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %+v", events)
	}
}

func TestSelfSkipped(t *testing.T) {
	w := newWatcher()
	w.tick(snapshot{State: "OPEN"})

	events, _ := w.tick(snapshot{State: "OPEN", Comments: []comment{newComment("comment", 1, "me", 0)}})
	if len(events) != 0 {
		t.Fatalf("events = %+v", events)
	}
}

func TestReviewFiltering(t *testing.T) {
	w := newWatcher()
	w.tick(snapshot{State: "OPEN"})

	empty := newComment("review", 1, "alice", 0)
	empty.State, empty.Body = "COMMENTED", ""
	pending := newComment("review", 2, "alice", 0)
	pending.State = "PENDING"
	approved := newComment("review", 3, "alice", 0)
	approved.State, approved.Body = "APPROVED", ""

	events, _ := w.tick(snapshot{State: "OPEN", Comments: []comment{empty, pending, approved}})
	if len(events) != 1 || events[0].(NewCommentEvent).ID != 3 {
		t.Fatalf("events = %+v", events)
	}
}

func TestClosedEmitsCommentsThenExits(t *testing.T) {
	w := newWatcher()
	w.tick(snapshot{State: "OPEN"})

	events, done := w.tick(snapshot{State: "MERGED", Comments: []comment{newComment("comment", 1, "alice", 0)}})
	if !done {
		t.Fatalf("want done")
	}
	if got := eventNames(events); !reflect.DeepEqual(got, []string{"new_comment", "pr_closed"}) {
		t.Fatalf("events = %v", got)
	}
	if events[1] != (PRClosedEvent{Merged: true}) {
		t.Fatalf("event = %+v", events[1])
	}
}

func TestAlreadyClosedExitsImmediately(t *testing.T) {
	w := newWatcher()
	events, done := w.tick(snapshot{State: "CLOSED", Comments: []comment{newComment("comment", 1, "alice", 0)}})
	if !done || !reflect.DeepEqual(eventNames(events), []string{"pr_closed"}) {
		t.Fatalf("events = %+v done = %v", events, done)
	}
}

func TestFetchResolvesSelfOnce(t *testing.T) {
	w := NewWatcher("123", 0)
	w.Log = io.Discard
	calls := 0
	w.fetchSelf = func(context.Context) (string, error) {
		calls++
		return "me", nil
	}
	w.fetch = func(context.Context, string) (snapshot, error) {
		return snapshot{State: "OPEN", Comments: []comment{newComment("comment", 1, "me", 0)}}, nil
	}

	for range 2 {
		if _, _, err := w.Fetch(context.Background()); err != nil {
			t.Fatalf("err = %v", err)
		}
	}
	if calls != 1 || w.self != "me" {
		t.Fatalf("calls = %d self = %q", calls, w.self)
	}
}
