// Package prcheck watches a PR's CI checks via `gh pr checks` and turns bucket
// transitions into monitor.Event: check_failed (once per failure) and
// checks_passed (once, when everything settles green).
package prcheck

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
)

const (
	DefaultInterval  = 30 * time.Second
	MaxFetchFailures = 5 // consecutive gh failures before the engine gives up
)

// Check mirrors one row of `gh pr checks --json name,bucket,link`.
type Check struct {
	Name   string `json:"name"`
	Bucket string `json:"bucket"`
	Link   string `json:"link"`
}

// See https://cli.github.com/manual/gh_pr_checks — bucket categorizes state into
// pass, fail, pending, skipping, or cancel.
var (
	green = map[string]bool{"pass": true, "skipping": true}
	red   = map[string]bool{"fail": true, "cancel": true}
)

var noChecksReported = regexp.MustCompile(`(?i)no checks reported`)

// FetchChecks shells out to gh. gh exits non-zero while checks fail or pend but
// still prints JSON, so the signal is parseable output, not exit status.
func FetchChecks(pr string) ([]Check, error) {
	var out, errBuf strings.Builder
	cmd := exec.Command("gh", "pr", "checks", pr, "--json", "name,bucket,link")
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	_ = cmd.Run()

	if strings.TrimSpace(out.String()) != "" {
		var checks []Check
		if err := json.Unmarshal([]byte(out.String()), &checks); err != nil {
			return nil, fmt.Errorf("unparseable gh output: %w", err)
		}
		return checks, nil
	}
	if noChecksReported.MatchString(errBuf.String()) {
		return []Check{}, nil
	}
	if strings.TrimSpace(errBuf.String()) == "" {
		return nil, fmt.Errorf("no output from gh pr checks")
	}
	return nil, fmt.Errorf("%s", strings.TrimSpace(errBuf.String()))
}

// Checker implements monitor.Prober[[]Check] for a single PR.
type Checker struct {
	PR  string
	Log io.Writer // where "no checks reported yet" goes; nil defaults to os.Stderr

	seenFailed map[[2]string]bool
}

func NewChecker(pr string) *Checker {
	return &Checker{PR: pr}
}

func (c *Checker) Fetch() ([]Check, error) {
	return FetchChecks(c.PR)
}

// Tick announces each check the moment it turns red, keyed by [name, run URL] so
// a fresh failing run re-reports even when the intervening pending/green state
// fell between polls and was never observed. All green is the one terminal state.
func (c *Checker) Tick(checks []Check) ([]monitor.Event, bool) {
	if len(checks) == 0 {
		fmt.Fprintln(c.logger(), "no checks reported yet; waiting…")
		return nil, false
	}

	if c.seenFailed == nil {
		c.seenFailed = map[[2]string]bool{}
	}

	var events []monitor.Event
	allGreen := true
	for _, ch := range checks {
		if red[ch.Bucket] {
			key := [2]string{ch.Name, ch.Link}
			if !c.seenFailed[key] {
				c.seenFailed[key] = true
				events = append(events, monitor.Event{
					Name: "check_failed",
					Data: map[string]any{"name": ch.Name, "url": ch.Link},
				})
			}
		}
		if !green[ch.Bucket] {
			allGreen = false
		}
	}

	if allGreen {
		events = append(events, monitor.Event{
			Name: "checks_passed",
			Data: map[string]any{"total": len(checks)},
		})
	}

	return events, allGreen
}

func (c *Checker) logger() io.Writer {
	if c.Log != nil {
		return c.Log
	}
	return os.Stderr
}
