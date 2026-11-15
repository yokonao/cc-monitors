// Package execmon runs an external command as a monitor, forwarding the event
// lines it prints.
package execmon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Run runs name with args, writing each stdout line that is an event to out
// and reporting other lines on errOut. The command's stderr goes to errOut.
// It returns the command's exit code.
func Run(ctx context.Context, name string, args []string, out, errOut io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 10 * time.Second
	cmd.Stderr = errOut
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	if err := cmd.Start(); err != nil {
		return 1, err
	}

	sc := bufio.NewScanner(stdout)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if err := checkEvent(line); err != nil {
			_, _ = fmt.Fprintf(errOut, "exec: dropped stdout line (%v): %s\n", err, line)
			continue
		}
		_, _ = fmt.Fprintln(out, line)
	}
	scanErr := sc.Err()
	if scanErr != nil {
		_ = cmd.Process.Kill()
	}

	err = cmd.Wait()
	if scanErr != nil {
		return 1, fmt.Errorf("read stdout: %w", scanErr)
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if code := exitErr.ExitCode(); code > 0 {
			return code, nil
		}
		return 1, err
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}

// checkEvent accepts a JSON object with a non-empty string event and an
// object data, the shape every cc-monitors monitor prints.
func checkEvent(line string) error {
	var e struct {
		Event *string          `json:"event"`
		Data  *json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		return errors.New("not a JSON object")
	}
	if e.Event == nil || *e.Event == "" {
		return errors.New("no event")
	}
	if e.Data == nil || !strings.HasPrefix(string(*e.Data), "{") {
		return errors.New("data is not an object")
	}
	return nil
}
