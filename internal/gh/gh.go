// Package gh runs the GitHub CLI.
package gh

import (
	"bytes"
	"context"
	"os/exec"
)

// Run executes gh and returns both streams and the exit error untouched;
// callers decide what a non-zero exit means.
func Run(ctx context.Context, args ...string) (stdout, stderr []byte, err error) {
	var out, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return out.Bytes(), errBuf.Bytes(), err
}
