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
	kindComment       = "comment"
	kindReview        = "review"
	kindReviewComment = "review_comment"
)

type snapshot struct {
	State    string // OPEN, CLOSED or MERGED
	Comments []comment
}

type comment struct {
	Kind      string
	ID        int64
	API       string // full REST URL; `gh api <API>` reads the latest state
	Author    string
	Body      string
	CreatedAt time.Time
	State     string // review only
}

// fetchSnapshot resolves pr (number, URL or branch) and reads its state plus
// all three comment kinds.
func fetchSnapshot(ctx context.Context, pr string) (snapshot, error) {
	out, err := gh(ctx, "pr", "view", pr, "--json", "state,url")
	if err != nil {
		return snapshot{}, err
	}
	var view struct {
		State string `json:"state"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(out, &view); err != nil {
		return snapshot{}, fmt.Errorf("unparseable gh output: %w", err)
	}

	u, err := url.Parse(view.URL)
	if err != nil {
		return snapshot{}, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/") // owner/repo/pull/N
	if len(parts) != 4 {
		return snapshot{}, fmt.Errorf("unexpected PR url %q", view.URL)
	}
	issue := "repos/" + parts[0] + "/" + parts[1] + "/issues/" + parts[3]
	pull := "repos/" + parts[0] + "/" + parts[1] + "/pulls/" + parts[3]

	snap := snapshot{State: view.State}
	for _, src := range []struct {
		path string
		list func(ctx context.Context, host, path string) ([]comment, error)
	}{
		{issue + "/comments", listComments[apiIssueComment]},
		{pull + "/reviews", listComments[apiReview]},
		{pull + "/comments", listComments[apiReviewComment]},
	} {
		cs, err := src.list(ctx, u.Host, src.path)
		if err != nil {
			return snapshot{}, err
		}
		snap.Comments = append(snap.Comments, cs...)
	}
	return snap, nil
}

func fetchSelf(ctx context.Context) (string, error) {
	out, err := gh(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type apiUser struct {
	Login string `json:"login"`
}

type apiIssueComment struct {
	ID        int64     `json:"id"`
	User      apiUser   `json:"user"`
	Body      string    `json:"body"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

func (a apiIssueComment) toComment() comment {
	return comment{Kind: kindComment, ID: a.ID, API: a.URL, Author: a.User.Login, Body: a.Body, CreatedAt: a.CreatedAt}
}

type apiReview struct {
	ID             int64     `json:"id"`
	User           apiUser   `json:"user"`
	Body           string    `json:"body"`
	PullRequestURL string    `json:"pull_request_url"`
	SubmittedAt    time.Time `json:"submitted_at"`
	State          string    `json:"state"`
}

// Reviews have no url of their own, so API is built from the PR's.
func (a apiReview) toComment() comment {
	return comment{
		Kind:      kindReview,
		ID:        a.ID,
		API:       fmt.Sprintf("%s/reviews/%d", a.PullRequestURL, a.ID),
		Author:    a.User.Login,
		Body:      a.Body,
		CreatedAt: a.SubmittedAt,
		State:     a.State,
	}
}

type apiReviewComment struct {
	ID        int64     `json:"id"`
	User      apiUser   `json:"user"`
	Body      string    `json:"body"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

func (a apiReviewComment) toComment() comment {
	return comment{Kind: kindReviewComment, ID: a.ID, API: a.URL, Author: a.User.Login, Body: a.Body, CreatedAt: a.CreatedAt}
}

func listComments[T interface{ toComment() comment }](ctx context.Context, host, path string) ([]comment, error) {
	items, err := ghAPIList[T](ctx, host, path)
	if err != nil {
		return nil, err
	}
	cs := make([]comment, len(items))
	for i, it := range items {
		cs[i] = it.toComment()
	}
	return cs, nil
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
