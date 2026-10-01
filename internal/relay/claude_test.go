package relay

import "testing"

func TestFindSession(t *testing.T) {
	agents := []agent{
		{SessionID: "aaaa1111-x", ID: "aaaa1111"},
		{SessionID: "aaaa1111-x", ID: "aaaa1111"},
		{SessionID: "bbbb2222-x", ID: "dup"},
		{SessionID: "cccc3333-x", ID: "dup"},
	}

	for _, s := range []string{"aaaa1111", "aaaa1111-x"} {
		if a, err := findSession(agents, s); err != nil || a.SessionID != "aaaa1111-x" {
			t.Fatalf("findSession(%q) = %+v, %v", s, a, err)
		}
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
