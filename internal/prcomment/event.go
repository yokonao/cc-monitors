package prcomment

import "github.com/yokonao/cc-monitors/internal/monitor"

// Every event this monitor emits. The three comment events share their name
// with Comment.Kind.
const (
	EventComment       = "comment"        // author, body, url
	EventReview        = "review"         // author, body, url, state
	EventReviewComment = "review_comment" // author, body, url, path, line, in_reply_to?
	EventPRClosed      = "pr_closed"      // merged
)

const MaxBodyRunes = 1000

func commentEvent(c Comment) monitor.Event {
	data := map[string]any{"author": c.Author, "body": truncate(c.Body), "url": c.URL}
	switch c.Kind {
	case EventReview:
		data["state"] = c.State
	case EventReviewComment:
		data["path"] = c.Path
		data["line"] = c.Line
		if c.InReplyTo != 0 {
			data["in_reply_to"] = c.InReplyTo
		}
	}
	return monitor.Event{Name: c.Kind, Data: data}
}

func closedEvent(merged bool) monitor.Event {
	return monitor.Event{Name: EventPRClosed, Data: map[string]any{"merged": merged}}
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) <= MaxBodyRunes {
		return s
	}
	return string(r[:MaxBodyRunes]) + "…"
}
