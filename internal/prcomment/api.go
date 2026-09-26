package prcomment

import (
	"context"
	"fmt"
	"time"
)

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
