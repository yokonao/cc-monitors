// Package gh runs the GitHub CLI.
package gh

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

// waitDelay bounds Run after ctx is done even when a process gh spawned still
// holds the output pipes; without it Run blocks until that process exits.
var waitDelay = 5 * time.Second

// Run executes gh and returns both streams and the exit error untouched;
// callers decide what a non-zero exit means.
func Run(ctx context.Context, args ...string) (stdout, stderr []byte, err error) {
	var out, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	cmd.WaitDelay = waitDelay
	err = cmd.Run()
	return out.Bytes(), errBuf.Bytes(), err
}
