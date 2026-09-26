// Package prcomment watches a PR's conversation comments, reviews and inline
// review comments via `gh`, emitting new_comment per new comment and pr_closed
// once the PR is merged or closed.
package prcomment

import (
	"context"
	"io"
	"os"
	"sort"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

const (
	DefaultInterval  = 30 * time.Second
	MaxFetchFailures = 5
)

// Watcher implements monitor.Prober for a single PR.
type Watcher struct {
	Interval         time.Duration
	MaxFetchFailures int
	Log              io.Writer

	fetch     func(ctx context.Context) (snapshot, error)
	fetchSelf func(ctx context.Context) (string, error)
	self      string
	seen      map[string]bool // keyed by comment.API; nil until the baseline poll
}

func NewWatcher(pr string, interval time.Duration) *Watcher {
	return &Watcher{
		Interval:  interval,
		fetch:     func(ctx context.Context) (snapshot, error) { return fetchSnapshot(ctx, pr) },
		fetchSelf: fetchSelf,
	}
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
	snap, err := monitor.Retry(ctx, max, w.Interval, w.logger(), w.fetch)
	if err != nil {
		return nil, false, err
	}
	events, done := w.tick(snap)
	return events, done, nil
}

// tick treats the first snapshot as a baseline and reports only comments that
// appear after it. The watcher's own comments are skipped so Claude's replies
// don't wake itself.
func (w *Watcher) tick(snap snapshot) ([]monitor.Event, bool) {
	baseline := w.seen == nil
	if baseline {
		w.seen = map[string]bool{}
	}

	var fresh []comment
	for _, c := range snap.Comments {
		if w.seen[c.API] {
			continue
		}
		w.seen[c.API] = true
		if !baseline && c.Author != w.self {
			fresh = append(fresh, c)
		}
	}
	sort.SliceStable(fresh, func(i, j int) bool { return fresh[i].CreatedAt.Before(fresh[j].CreatedAt) })

	var events []monitor.Event
	for _, c := range fresh {
		events = append(events, NewCommentEvent{Kind: c.Kind, ID: c.ID, API: c.API})
	}

	if snap.State != "OPEN" {
		events = append(events, PRClosedEvent{Merged: snap.State == "MERGED"})
		return events, true
	}
	return events, false
}

func (w *Watcher) logger() io.Writer {
	if w.Log != nil {
		return w.Log
	}
	return os.Stderr
}
