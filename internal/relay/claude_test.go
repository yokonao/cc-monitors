package relay

import (
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
