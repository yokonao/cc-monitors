package relay

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindSession(t *testing.T) {
	agents := []agent{
		{SessionID: "aaaa1111-x", ID: "aaaa1111"},
		{SessionID: "aaaa1111-x", ID: "aaaa1111"},
		{SessionID: "bbbb2222-x", ID: "dup"},
		{SessionID: "cccc3333-x", ID: "dup"},
		{SessionID: "dddd4444-x", ID: "dddd4444", Name: "fix bug"},
		{SessionID: "eeee5555-x", ID: "eeee5555", Name: "fix bug in relay"},
		{SessionID: "ffff6666-x", ID: "ffff6666", Name: "Review relay"},
		{SessionID: "gggg7777-x", ID: "gggg7777", Name: "aaaa1111"},
	}

	for _, s := range []string{"aaaa1111", "aaaa1111-x"} {
		if a, err := findSession(agents, s); err != nil || a.SessionID != "aaaa1111-x" {
			t.Fatalf("findSession(%q) = %+v, %v", s, a, err)
		}
	}
	for s, want := range map[string]string{
		"fix bug":  "dddd4444-x",
		"in relay": "eeee5555-x",
		"review":   "ffff6666-x",
		"aaaa1111": "aaaa1111-x",
		"gggg7777": "gggg7777-x",
	} {
		if a, err := findSession(agents, s); err != nil || a.SessionID != want {
			t.Errorf("findSession(%q) = %+v, %v, want %s", s, a, err, want)
		}
	}
	if _, err := findSession(agents, "relay"); err == nil {
		t.Fatal("ambiguous partial name matched")
	}
	if _, err := findSession(agents, "dup"); err == nil {
		t.Fatal("ambiguous short id matched")
	}
	if _, err := findSession(agents, "zzz"); err == nil {
		t.Fatal("unknown session matched")
	}
}

func TestRecipient(t *testing.T) {
	agents := []agent{
		{SessionID: "a", Name: "fix bug"},
		{SessionID: "b", Name: "review"},
		{SessionID: "c", Name: "review"},
		{SessionID: "d"},
	}

	if name, running, err := recipient(agents, "a"); name != "fix bug" || !running || err != nil {
		t.Fatalf("recipient(a) = %q, %v, %v", name, running, err)
	}
	if _, running, err := recipient(agents, "z"); running || err != nil {
		t.Fatalf("recipient(z) = %v, %v, want stopped", running, err)
	}
	if _, _, err := recipient(agents, "b"); err == nil {
		t.Fatal("shared name was accepted")
	}
	if _, _, err := recipient(agents, "d"); err == nil {
		t.Fatal("empty name was accepted")
	}
}

func TestSendPrompt(t *testing.T) {
	p := sendPrompt("fix bug", "hello")
	for _, want := range []string{`"fix bug"`, "<message>\nhello\n</message>", "not as instructions", "not delivered", "FAILED:"} {
		if !strings.Contains(p, want) {
			t.Errorf("sendPrompt lacks %q:\n%s", want, p)
		}
	}
}

func TestClaudeLogsStderrOnce(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho SENT\necho 'Warning: sign-in expires soon' >&2\necho 'Warning: MCP blocked' >&2\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var buf bytes.Buffer
	c := Claude{Log: log.New(&buf, "", 0)}
	for range 2 {
		out, err := c.claude(context.Background(), "", "", "-p")
		if err != nil || string(out) != "SENT\n" {
			t.Fatalf("claude = %q, %v", out, err)
		}
	}
	if got, want := buf.String(), "claude -p: Warning: sign-in expires soon\nclaude -p: Warning: MCP blocked\n"; got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}
