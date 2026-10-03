package prmerge

// PRClosedEvent matches pr-comments' pr_closed, so a session handles both alike.
type PRClosedEvent struct {
	Merged bool `json:"merged"`
}

func (PRClosedEvent) EventName() string { return "pr_closed" }
