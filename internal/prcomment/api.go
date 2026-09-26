package prcomment

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type apiUser struct {
	Login string `json:"login"`
}

type apiIssueComment struct {
	ID        int64     `json:"id"`
	User      apiUser   `json:"user"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

func (a apiIssueComment) toComment() (comment, bool) {
	return comment{Kind: kindComment, ID: a.ID, API: a.URL, Author: a.User.Login, CreatedAt: a.CreatedAt}, true
}

type apiReview struct {
	ID             int64     `json:"id"`
	User           apiUser   `json:"user"`
	Body           string    `json:"body"`
	PullRequestURL string    `json:"pull_request_url"`
	SubmittedAt    time.Time `json:"submitted_at"`
	State          string    `json:"state"`
}

// Reviews have no url of their own, so API is built from the PR's. Pending
// reviews and bodiless COMMENTED ones are dropped: the former are unsubmitted,
// the latter's content arrives as review comments.
func (a apiReview) toComment() (comment, bool) {
	if a.State == "PENDING" || (a.State == "COMMENTED" && strings.TrimSpace(a.Body) == "") {
		return comment{}, false
	}
	return comment{
		Kind:      kindReview,
		ID:        a.ID,
		API:       fmt.Sprintf("%s/reviews/%d", a.PullRequestURL, a.ID),
		Author:    a.User.Login,
		CreatedAt: a.SubmittedAt,
	}, true
}

type apiReviewComment struct {
	ID        int64     `json:"id"`
	User      apiUser   `json:"user"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

func (a apiReviewComment) toComment() (comment, bool) {
	return comment{Kind: kindReviewComment, ID: a.ID, API: a.URL, Author: a.User.Login, CreatedAt: a.CreatedAt}, true
}

// apiItem is one element of a REST list response; toComment reports false for
// items that should never be announced.
type apiItem interface {
	toComment() (comment, bool)
}

func listComments[T apiItem](ctx context.Context, host, path string) ([]comment, error) {
	items, err := ghAPIList[T](ctx, host, path)
	if err != nil {
		return nil, err
	}
	var cs []comment
	for _, it := range items {
		if c, ok := it.toComment(); ok {
			cs = append(cs, c)
		}
	}
	return cs, nil
}
