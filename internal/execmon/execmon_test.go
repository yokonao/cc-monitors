package execmon

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
)

// syncBuffer is shared by Run and the goroutine copying the command's stderr.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func run(t *testing.T, script string) (int, string, string) {
	t.Helper()
	var out bytes.Buffer
	var errOut syncBuffer
	code, err := Run(context.Background(), "sh", []string{"-c", script}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return code, out.String(), errOut.String()
}

func TestRunForwardsOnlyEvents(t *testing.T) {
	code, out, errOut := run(t, `
echo '{"event":"a","data":{"n":1}}'
echo 'debug output'
echo
echo '{"event":"b","data":1}'
echo '{"data":{}}'
echo '  {"data":{},"event":"c"}  '
echo 'to stderr' >&2
`)
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if want := "{\"event\":\"a\",\"data\":{\"n\":1}}\n{\"data\":{},\"event\":\"c\"}\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	for _, want := range []string{"dropped stdout line (not a JSON object): debug output", "(data is not an object)", "(no event)", "to stderr"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
}

func TestRunReturnsExitCode(t *testing.T) {
	if code, _, _ := run(t, "exit 3"); code != 3 {
		t.Fatalf("code = %d, want 3", code)
	}
}

func TestRunMissingCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code, err := Run(context.Background(), "/nonexistent/cmd", nil, &out, &errOut)
	if code == 0 || err == nil {
		t.Fatalf("Run = %d, %v", code, err)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	done := make(chan int)
	go func() {
		code, _ := Run(ctx, "sh", []string{"-c", `echo '{"event":"a","data":{}}'; exec sleep 60`}, &out, &bytes.Buffer{})
		done <- code
	}()
	cancel()
	if code := <-done; code == 0 {
		t.Fatal("cancelled run reported success")
	}
}
