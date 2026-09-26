package prcheck

type CheckFailedEvent struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (CheckFailedEvent) EventName() string { return "check_failed" }

type ChecksPassedEvent struct {
	Total int `json:"total"`
}

func (ChecksPassedEvent) EventName() string { return "checks_passed" }
