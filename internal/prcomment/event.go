package prcomment

// NewCommentEvent carries only where to look: the woken Claude reads the
// latest content itself with `gh api <API>`.
type NewCommentEvent struct {
	Kind string `json:"kind"` // comment, review or review_comment
	ID   int64  `json:"id"`
	API  string `json:"api"`
}

func (NewCommentEvent) EventName() string { return "new_comment" }

type PRClosedEvent struct {
	Merged bool `json:"merged"`
}

func (PRClosedEvent) EventName() string { return "pr_closed" }
