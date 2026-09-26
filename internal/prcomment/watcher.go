// Package prcomment watches a PR's conversation comments, reviews and inline
// review comments via `gh`, emitting one monitor.Event per new comment and
// pr_closed once the PR is merged or closed.
package prcomment

import (
	"context"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

const (
	DefaultInterval  = 30 * time.Second
	MaxFetchFailures = 5
	MaxBodyRunes     = 1000
)

// Watcher implements monitor.Prober for a single PR.
type Watcher struct {
	PR               string
	Interval         time.Duration
	MaxFetchFailures int
	Log              io.Writer

	fetch     func(ctx context.Context, pr string) (Snapshot, error)
	fetchSelf func(ctx context.Context) (string, error)
	self      string
	seen      map[[2]any]bool // nil until the baseline poll
}

func NewWatcher(pr string, interval time.Duration) *Watcher {
	return &Watcher{PR: pr, Interval: interval, fetch: FetchSnapshot, fetchSelf: FetchSelf}
}

func (w *Watcher) Fetch(ctx context.Context) ([]monitor.Event, bool, error) {
	max := w.MaxFetchFailures
	if max <= 0 {
		max = MaxFetchFailures
	}
	if w.self == "" {
		self, err := monitor.Retry(ctx, max, w.Interval, w.logger(), w.fetchSelf)
		if err != nil {
			return nil, false, err
		}
		w.self = self
	}
	snap, err := monitor.Retry(ctx, max, w.Interval, w.logger(), func(ctx context.Context) (Snapshot, error) {
		return w.fetch(ctx, w.PR)
	})
	if err != nil {
		return nil, false, err
	}
	events, done := w.Tick(snap)
	return events, done, nil
}

// Tick treats the first snapshot as a baseline and reports only comments that
// appear after it. The watcher's own comments are skipped so Claude's replies
// don't wake itself.
func (w *Watcher) Tick(snap Snapshot) ([]monitor.Event, bool) {
	baseline := w.seen == nil
	if baseline {
		w.seen = map[[2]any]bool{}
	}

	var fresh []Comment
	for _, c := range snap.Comments {
		key := [2]any{c.Kind, c.ID}
		if w.seen[key] {
			continue
		}
		w.seen[key] = true
		if !baseline && w.notable(c) {
			fresh = append(fresh, c)
		}
	}
	sort.SliceStable(fresh, func(i, j int) bool { return fresh[i].CreatedAt.Before(fresh[j].CreatedAt) })

	var events []monitor.Event
	for _, c := range fresh {
		events = append(events, commentEvent(c))
	}

	if snap.State != "OPEN" {
		events = append(events, PRClosedEvent{Merged: snap.State == "MERGED"})
		return events, true
	}
	return events, false
}

func (w *Watcher) notable(c Comment) bool {
	if c.Author == w.self {
		return false
	}
	if c.Kind == KindReview {
		switch c.State {
		case "PENDING":
			return false
		case "COMMENTED":
			// The inline review_comments carry the content.
			return strings.TrimSpace(c.Body) != ""
		}
	}
	return true
}

func commentEvent(c Comment) monitor.Event {
	body := truncate(c.Body)
	switch c.Kind {
	case KindReview:
		return ReviewEvent{Author: c.Author, Body: body, URL: c.URL, State: c.State}
	case KindReviewComment:
		return ReviewCommentEvent{Author: c.Author, Body: body, URL: c.URL, Path: c.Path, Line: c.Line, InReplyTo: c.InReplyTo}
	}
	return CommentEvent{Author: c.Author, Body: body, URL: c.URL}
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) <= MaxBodyRunes {
		return s
	}
	return string(r[:MaxBodyRunes]) + "…"
}

func (w *Watcher) logger() io.Writer {
	if w.Log != nil {
		return w.Log
	}
	return os.Stderr
}
