package prcomment

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func comment(kind string, id int64, author string, min int) Comment {
	return Comment{Kind: kind, ID: id, Author: author, Body: "hi", URL: "https://gh.test/" + kind, CreatedAt: t0.Add(time.Duration(min) * time.Minute)}
}

func newWatcher() *Watcher {
	w := NewWatcher("123", 0)
	w.self = "me"
	return w
}

func eventNames(events []monitor.Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Name)
	}
	return out
}

func TestBaselineIsSilent(t *testing.T) {
	w := newWatcher()
	events, done := w.Tick(Snapshot{State: "OPEN", Comments: []Comment{comment("comment", 1, "alice", 0)}})
	if done || len(events) != 0 {
		t.Fatalf("events = %+v done = %v", events, done)
	}

	events, _ = w.Tick(Snapshot{State: "OPEN", Comments: []Comment{comment("comment", 1, "alice", 0)}})
	if len(events) != 0 {
		t.Fatalf("re-poll should emit nothing, got %+v", events)
	}
}

func TestNewCommentsEmittedInOrder(t *testing.T) {
	w := newWatcher()
	w.Tick(Snapshot{State: "OPEN"})

	events, done := w.Tick(Snapshot{State: "OPEN", Comments: []Comment{
		comment("review_comment", 1, "alice", 2),
		comment("comment", 1, "bob", 1),
	}})
	if done {
		t.Fatalf("want not done")
	}
	if got := eventNames(events); !reflect.DeepEqual(got, []string{"comment", "review_comment"}) {
		t.Fatalf("events = %v", got)
	}
	want := map[string]any{"author": "bob", "body": "hi", "url": "https://gh.test/comment"}
	if !reflect.DeepEqual(events[0].Data, want) {
		t.Fatalf("data = %+v", events[0].Data)
	}
}

func TestSelfSkipped(t *testing.T) {
	w := newWatcher()
	w.Tick(Snapshot{State: "OPEN"})

	events, _ := w.Tick(Snapshot{State: "OPEN", Comments: []Comment{comment("comment", 1, "me", 0)}})
	if len(events) != 0 {
		t.Fatalf("events = %+v", events)
	}
}

func TestReviewFiltering(t *testing.T) {
	w := newWatcher()
	w.Tick(Snapshot{State: "OPEN"})

	empty := comment("review", 1, "alice", 0)
	empty.State, empty.Body = "COMMENTED", ""
	pending := comment("review", 2, "alice", 0)
	pending.State = "PENDING"
	approved := comment("review", 3, "alice", 0)
	approved.State, approved.Body = "APPROVED", ""

	events, _ := w.Tick(Snapshot{State: "OPEN", Comments: []Comment{empty, pending, approved}})
	if len(events) != 1 || events[0].Data["state"] != "APPROVED" {
		t.Fatalf("events = %+v", events)
	}
}

func TestReviewCommentData(t *testing.T) {
	w := newWatcher()
	w.Tick(Snapshot{State: "OPEN"})

	c := comment("review_comment", 1, "alice", 0)
	c.Path, c.Line, c.InReplyTo = "main.go", 42, 7
	events, _ := w.Tick(Snapshot{State: "OPEN", Comments: []Comment{c}})
	want := map[string]any{"author": "alice", "body": "hi", "url": "https://gh.test/review_comment", "path": "main.go", "line": 42, "in_reply_to": int64(7)}
	if len(events) != 1 || !reflect.DeepEqual(events[0].Data, want) {
		t.Fatalf("events = %+v", events)
	}
}

func TestLongBodyTruncated(t *testing.T) {
	w := newWatcher()
	w.Tick(Snapshot{State: "OPEN"})

	c := comment("comment", 1, "alice", 0)
	c.Body = strings.Repeat("あ", MaxBodyRunes+1)
	events, _ := w.Tick(Snapshot{State: "OPEN", Comments: []Comment{c}})
	if got := events[0].Data["body"].(string); got != strings.Repeat("あ", MaxBodyRunes)+"…" {
		t.Fatalf("body len = %d", len([]rune(got)))
	}
}

func TestClosedEmitsCommentsThenExits(t *testing.T) {
	w := newWatcher()
	w.Tick(Snapshot{State: "OPEN"})

	events, done := w.Tick(Snapshot{State: "MERGED", Comments: []Comment{comment("comment", 1, "alice", 0)}})
	if !done {
		t.Fatalf("want done")
	}
	if got := eventNames(events); !reflect.DeepEqual(got, []string{"comment", "pr_closed"}) {
		t.Fatalf("events = %v", got)
	}
	if events[1].Data["merged"] != true {
		t.Fatalf("data = %+v", events[1].Data)
	}
}

func TestAlreadyClosedExitsImmediately(t *testing.T) {
	w := newWatcher()
	events, done := w.Tick(Snapshot{State: "CLOSED", Comments: []Comment{comment("comment", 1, "alice", 0)}})
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
	w.fetch = func(context.Context, string) (Snapshot, error) {
		return Snapshot{State: "OPEN", Comments: []Comment{comment("comment", 1, "me", 0)}}, nil
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
