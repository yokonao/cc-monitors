package prcomment

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/url"
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
	CreatedAt time.Time
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
