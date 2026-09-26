package prcomment

type CommentEvent struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

func (CommentEvent) EventName() string { return "comment" }

type ReviewEvent struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

func (ReviewEvent) EventName() string { return "review" }

type ReviewCommentEvent struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	InReplyTo int64  `json:"in_reply_to,omitzero"`
}

func (ReviewCommentEvent) EventName() string { return "review_comment" }

type PRClosedEvent struct {
	Merged bool `json:"merged"`
}

func (PRClosedEvent) EventName() string { return "pr_closed" }
