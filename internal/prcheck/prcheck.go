// Package prcheck watches a PR's CI checks via `gh pr checks` and turns bucket
// transitions into monitor.Event: check_failed (once per failure) and
// checks_passed (once, when everything settles green).
package prcheck

import (
	"context"
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
	MaxFetchFailures = 5 // consecutive gh failures before Checker.Fetch gives up
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

// FetchChecks shells out to gh once. gh exits non-zero while checks fail or
// pend but still prints JSON, so the signal is parseable output, not exit
// status.
func FetchChecks(ctx context.Context, pr string) ([]Check, error) {
	var out, errBuf strings.Builder
	cmd := exec.CommandContext(ctx, "gh", "pr", "checks", pr, "--json", "name,bucket,link")
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()

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
	if msg := strings.TrimSpace(errBuf.String()); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	// gh printed nothing at all (to either stream) — surface why it didn't run
	// (missing binary, killed by ctx, ...) instead of a generic message.
	if runErr != nil {
		return nil, fmt.Errorf("gh pr checks: %w", runErr)
	}
	return nil, fmt.Errorf("no output from gh pr checks")
}

// Checker implements monitor.Prober for a single PR.
type Checker struct {
	PR               string
	Interval         time.Duration // wait between retries, and between polls
	MaxFetchFailures int           // 0 means MaxFetchFailures
	Log              io.Writer     // retry/give-up logging; nil defaults to os.Stderr

	fetch      func(ctx context.Context, pr string) ([]Check, error)
	seenFailed map[[2]string]bool
}

func NewChecker(pr string, interval time.Duration) *Checker {
	return &Checker{PR: pr, Interval: interval, fetch: FetchChecks}
}

// Fetch implements monitor.Prober. It retries gh internally, waiting Interval
// between attempts, up to MaxFetchFailures consecutive failures before giving
// up — the engine sees at most one terminal error per poll.
func (c *Checker) Fetch(ctx context.Context) ([]monitor.Event, bool, error) {
	checks, err := c.fetchWithRetry(ctx)
	if err != nil {
		return nil, false, err
	}

	events, done := c.Tick(checks)
	return events, done, nil
}

func (c *Checker) fetchWithRetry(ctx context.Context) ([]Check, error) {
	max := c.MaxFetchFailures
	if max <= 0 {
		max = MaxFetchFailures
	}

	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		checks, err := c.fetch(ctx, c.PR)
		if err == nil {
			return checks, nil
		}
		lastErr = err
		fmt.Fprintf(c.logger(), "fetch failed (%d/%d): %v\n", attempt, max, err)
		if attempt < max {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.Interval):
			}
		}
	}
	fmt.Fprintf(c.logger(), "giving up after %d consecutive fetch failures\n", max)
	return nil, lastErr
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
