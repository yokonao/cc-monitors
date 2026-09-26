// Package prcomment watches a PR's conversation comments, reviews and inline
// review comments via `gh`, emitting one monitor.Event per new comment and
// pr_closed once the PR is merged or closed.
package prcomment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
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

type Comment struct {
	Kind      string // comment, review or review_comment
	ID        int64
	Author    string
	Body      string
	URL       string
	CreatedAt time.Time
	State     string // review only
	Path      string // review_comment only
	Line      int    // review_comment only
	InReplyTo int64  // review_comment only
}

type Snapshot struct {
	State    string // OPEN, CLOSED or MERGED
	Comments []Comment
}

func gh(ctx context.Context, args ...string) ([]byte, error) {
	var out, errBuf strings.Builder
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errBuf.String()); msg != "" {
			return nil, fmt.Errorf("gh %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("gh %s: %w", args[0], err)
	}
	return []byte(out.String()), nil
}

func FetchSelf(ctx context.Context) (string, error) {
	out, err := gh(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ghAPIList pages through a REST list endpoint, decoding one element per line.
func ghAPIList[T any](ctx context.Context, host, path string) ([]T, error) {
	out, err := gh(ctx, "api", "--hostname", host, "--paginate", "--jq", ".[]", path)
	if err != nil {
		return nil, err
	}
	var items []T
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var v T
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("unparseable gh output: %w", err)
		}
		items = append(items, v)
	}
	return items, nil
}

type apiComment struct {
	ID           int64                  `json:"id"`
	User         struct{ Login string } `json:"user"`
	Body         string                 `json:"body"`
	HTMLURL      string                 `json:"html_url"`
	CreatedAt    time.Time              `json:"created_at"`
	SubmittedAt  time.Time              `json:"submitted_at"`
	State        string                 `json:"state"`
	Path         string                 `json:"path"`
	Line         *int                   `json:"line"`
	OriginalLine *int                   `json:"original_line"`
	InReplyToID  int64                  `json:"in_reply_to_id"`
}

func (a apiComment) toComment(kind string) Comment {
	c := Comment{
		Kind:      kind,
		ID:        a.ID,
		Author:    a.User.Login,
		Body:      a.Body,
		URL:       a.HTMLURL,
		CreatedAt: a.CreatedAt,
		State:     a.State,
		Path:      a.Path,
		InReplyTo: a.InReplyToID,
	}
	if kind == "review" {
		c.CreatedAt = a.SubmittedAt
	}
	// line is null once the diff moves past the comment.
	if a.Line != nil {
		c.Line = *a.Line
	} else if a.OriginalLine != nil {
		c.Line = *a.OriginalLine
	}
	return c
}

// FetchSnapshot resolves pr (number, URL or branch) and reads its state plus
// all three comment kinds.
func FetchSnapshot(ctx context.Context, pr string) (Snapshot, error) {
	out, err := gh(ctx, "pr", "view", pr, "--json", "state,url")
	if err != nil {
		return Snapshot{}, err
	}
	var view struct{ State, URL string }
	if err := json.Unmarshal(out, &view); err != nil {
		return Snapshot{}, fmt.Errorf("unparseable gh output: %w", err)
	}

	u, err := url.Parse(view.URL)
	if err != nil {
		return Snapshot{}, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/") // owner/repo/pull/N
	if len(parts) != 4 {
		return Snapshot{}, fmt.Errorf("unexpected PR url %q", view.URL)
	}
	repo, num := parts[0]+"/"+parts[1], parts[3]

	snap := Snapshot{State: view.State}
	for _, src := range []struct{ kind, path string }{
		{"comment", "repos/" + repo + "/issues/" + num + "/comments"},
		{"review", "repos/" + repo + "/pulls/" + num + "/reviews"},
		{"review_comment", "repos/" + repo + "/pulls/" + num + "/comments"},
	} {
		items, err := ghAPIList[apiComment](ctx, u.Host, src.path)
		if err != nil {
			return Snapshot{}, err
		}
		for _, it := range items {
			snap.Comments = append(snap.Comments, it.toComment(src.kind))
		}
	}
	return snap, nil
}

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
		events = append(events, toEvent(c))
	}

	if snap.State != "OPEN" {
		events = append(events, monitor.Event{
			Name: "pr_closed",
			Data: map[string]any{"merged": snap.State == "MERGED"},
		})
		return events, true
	}
	return events, false
}

func (w *Watcher) notable(c Comment) bool {
	if c.Author == w.self {
		return false
	}
	if c.Kind == "review" {
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

func toEvent(c Comment) monitor.Event {
	data := map[string]any{"author": c.Author, "body": truncate(c.Body), "url": c.URL}
	switch c.Kind {
	case "review":
		data["state"] = c.State
	case "review_comment":
		data["path"] = c.Path
		data["line"] = c.Line
		if c.InReplyTo != 0 {
			data["in_reply_to"] = c.InReplyTo
		}
	}
	return monitor.Event{Name: c.Kind, Data: data}
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
