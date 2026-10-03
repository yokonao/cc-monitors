// Package prmerge watches a PR's state via `gh pr view`, emitting pr_closed
// once it is merged or closed.
package prmerge

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

const (
	DefaultInterval  = time.Minute
	MaxFetchFailures = 5
)

// Watcher implements monitor.Prober for a single PR.
type Watcher struct {
	Interval         time.Duration
	MaxFetchFailures int
	Log              io.Writer

	fetch func(ctx context.Context) (string, error)
}

func NewWatcher(pr string, interval time.Duration) *Watcher {
	return &Watcher{
		Interval: interval,
		fetch:    func(ctx context.Context) (string, error) { return fetchState(ctx, pr) },
	}
}

// Fetch reports a PR that is already closed at startup too, so a watch
// registered late doesn't miss the merge.
func (w *Watcher) Fetch(ctx context.Context) ([]monitor.Event, bool, error) {
	attempts := w.MaxFetchFailures
	if attempts <= 0 {
		attempts = MaxFetchFailures
	}
	state, err := monitor.Retry(ctx, attempts, w.Interval, w.logger(), w.fetch)
	if err != nil {
		return nil, false, err
	}
	if state == "OPEN" {
		return nil, false, nil
	}
	return []monitor.Event{PRClosedEvent{Merged: state == "MERGED"}}, true, nil
}

// fetchState returns OPEN, CLOSED or MERGED.
func fetchState(ctx context.Context, pr string) (string, error) {
	var out, errBuf strings.Builder
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", pr, "--json", "state", "--jq", ".state")
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errBuf.String()); msg != "" {
			return "", fmt.Errorf("gh pr view: %s", msg)
		}
		return "", fmt.Errorf("gh pr view: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

func (w *Watcher) logger() io.Writer {
	if w.Log != nil {
		return w.Log
	}
	return os.Stderr
}
