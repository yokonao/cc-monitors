// Package gh runs the GitHub CLI.
package gh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes gh and returns both streams and the exit error untouched, for
// callers that must read stdout even when gh exits non-zero.
func Run(ctx context.Context, args ...string) (stdout, stderr []byte, err error) {
	var out, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return out.Bytes(), errBuf.Bytes(), err
}

// Output executes gh and turns a non-zero exit into an error carrying gh's
// stderr.
func Output(ctx context.Context, args ...string) ([]byte, error) {
	out, errOut, err := Run(ctx, args...)
	if err != nil {
		if msg := strings.TrimSpace(string(errOut)); msg != "" {
			return nil, fmt.Errorf("gh %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("gh %s: %w", args[0], err)
	}
	return out, nil
}
