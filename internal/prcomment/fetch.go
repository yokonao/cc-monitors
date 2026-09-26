package prcomment

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/url"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const (
	KindComment       = "comment"
	KindReview        = "review"
	KindReviewComment = "review_comment"
)

type Snapshot struct {
	State    string // OPEN, CLOSED or MERGED
	Comments []Comment
}

type Comment struct {
	Kind      string
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

// FetchSnapshot resolves pr (number, URL or branch) and reads its state plus
// all three comment kinds.
func FetchSnapshot(ctx context.Context, pr string) (Snapshot, error) {
	out, err := gh(ctx, "pr", "view", pr, "--json", "state,url")
	if err != nil {
		return Snapshot{}, err
	}
	var view struct {
		State string `json:"state"`
		URL   string `json:"url"`
	}
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
		{KindComment, "repos/" + repo + "/issues/" + num + "/comments"},
		{KindReview, "repos/" + repo + "/pulls/" + num + "/reviews"},
		{KindReviewComment, "repos/" + repo + "/pulls/" + num + "/comments"},
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

func FetchSelf(ctx context.Context) (string, error) {
	out, err := gh(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type apiComment struct {
	ID   int64 `json:"id"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	Body         string    `json:"body"`
	HTMLURL      string    `json:"html_url"`
	CreatedAt    time.Time `json:"created_at"`
	SubmittedAt  time.Time `json:"submitted_at"`
	State        string    `json:"state"`
	Path         string    `json:"path"`
	Line         *int      `json:"line"`
	OriginalLine *int      `json:"original_line"`
	InReplyToID  int64     `json:"in_reply_to_id"`
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
	if kind == KindReview {
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

// ghAPIList pages through a REST list endpoint and flattens the pages.
func ghAPIList[T any](ctx context.Context, host, path string) ([]T, error) {
	out, err := gh(ctx, "api", "--hostname", host, "--paginate", "--slurp", path)
	if err != nil {
		return nil, err
	}
	var pages [][]T
	if err := json.Unmarshal(out, &pages); err != nil {
		return nil, fmt.Errorf("unparseable gh output: %w", err)
	}
	return slices.Concat(pages...), nil
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
