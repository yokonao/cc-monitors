package relay

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

type fakeSessions struct {
	delivered chan string
	failures  int // Deliver fails this many times first
}

func (f *fakeSessions) Resolve(_ context.Context, session string) (Session, error) {
	if session == "missing" {
		return Session{}, errors.New("not found")
	}
	return Session{ID: session + "-full", ShortID: session}, nil
}

func (f *fakeSessions) Deliver(_ context.Context, _ Session, text string) error {
	if f.failures > 0 {
		f.failures--
		return errors.New("boom")
	}
	f.delivered <- text
	return nil
}

// newTestServer runs each relay's monitor as `sh -c <first arg>`.
func newTestServer(t *testing.T, sessions *fakeSessions) *Server {
	t.Helper()
	return &Server{
		StatePath: filepath.Join(t.TempDir(), "relays.json"),
		Sessions:  sessions,
		Validate: func(monitor string, args []string) ([]string, error) {
			if monitor != "sh" {
				return nil, errors.New("unknown monitor")
			}
			return args[:1], nil
		},
		Command: func(ctx context.Context, r Relay) *exec.Cmd {
			return exec.CommandContext(ctx, "sh", "-c", r.Args[0])
		},
		Log:              log.New(io.Discard, "", 0),
		DeliveryAttempts: 3,
		DeliveryWait:     time.Millisecond,
		BackoffMin:       time.Millisecond,
		BackoffMax:       time.Millisecond,
		MaxFailures:      2,
		ResetAfter:       time.Hour,
	}
}

func startServer(t *testing.T, s *Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		s.wg.Wait()
	})
}

func add(t *testing.T, s *Server, args ...string) string {
	t.Helper()
	resp := s.handle(Request{Op: "add", Session: "abc", Monitor: "sh", Args: args})
	if resp.Error != "" {
		t.Fatal(resp.Error)
	}
	return resp.ID
}

func receive(t *testing.T, f *fakeSessions, n int) []string {
	t.Helper()
	var got []string
	for range n {
		select {
		case s := <-f.delivered:
			got = append(got, s)
		case <-time.After(5 * time.Second):
			t.Fatalf("got %q, want %d notices", got, n)
		}
	}
	return got
}

func waitRemoved(t *testing.T, s *Server) {
	t.Helper()
	for range 500 {
		if len(s.list("")) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("relay was not removed")
}

func TestRelaysEventsAndRemovesFinishedRelay(t *testing.T) {
	f := &fakeSessions{delivered: make(chan string, 10)}
	s := newTestServer(t, f)
	startServer(t, s)

	add(t, s, `echo '{"event":"x"}'`)

	want := []string{
		`[cc-monitors relay] sh echo '{"event":"x"}': {"event":"x"}`,
		`[cc-monitors relay] sh echo '{"event":"x"}': watch finished.`,
	}
	if got := receive(t, f, 2); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	waitRemoved(t, s)
	if state, _ := loadState(s.StatePath); len(state) != 0 {
		t.Fatalf("state = %v, want empty", state)
	}
}

func TestAddReturnsExistingRelayForSameTarget(t *testing.T) {
	f := &fakeSessions{delivered: make(chan string, 10)}
	s := newTestServer(t, f)
	startServer(t, s)

	id := add(t, s, "sleep 60", "--interval", "1m")
	if again := add(t, s, "sleep 60", "--interval", "2m"); again != id {
		t.Fatalf("id = %q, want %q", again, id)
	}
	relays := s.list("abc")
	if len(relays) != 1 || !slices.Equal(relays[0].Args, []string{"sleep 60", "--interval", "2m"}) {
		t.Fatalf("relays = %+v", relays)
	}
	if other := add(t, s, "sleep 61"); other == id {
		t.Fatal("a different target got the same relay ID")
	}

	state, err := loadState(s.StatePath)
	if err != nil || len(state) != 2 {
		t.Fatalf("state = %v, %v", state, err)
	}
}

func TestAddRejectsInvalidRegistration(t *testing.T) {
	s := newTestServer(t, &fakeSessions{})
	startServer(t, s)

	if resp := s.handle(Request{Op: "add", Session: "abc", Monitor: "nope", Args: []string{"x"}}); resp.Error == "" {
		t.Fatal("unknown monitor was accepted")
	}
	if resp := s.handle(Request{Op: "add", Session: "missing", Monitor: "sh", Args: []string{"x"}}); resp.Error == "" {
		t.Fatal("missing session was accepted")
	}
}

func TestRemoveAndList(t *testing.T) {
	s := newTestServer(t, &fakeSessions{})
	startServer(t, s)

	id := add(t, s, "sleep 60")
	if got := s.list("abc-full"); len(got) != 1 {
		t.Fatalf("list by session ID = %v", got)
	}
	if got := s.list("other"); len(got) != 0 {
		t.Fatalf("list by other session = %v", got)
	}
	if resp := s.handle(Request{Op: "rm", ID: id}); resp.Error != "" {
		t.Fatal(resp.Error)
	}
	if resp := s.handle(Request{Op: "rm", ID: id}); resp.Error == "" {
		t.Fatal("removing twice succeeded")
	}
	if got := s.list(""); len(got) != 0 {
		t.Fatalf("list = %v", got)
	}
}

func TestRemovesRelayAfterRepeatedFailures(t *testing.T) {
	f := &fakeSessions{delivered: make(chan string, 10)}
	s := newTestServer(t, f)
	startServer(t, s)

	add(t, s, "exit 1")

	want := []string{
		"[cc-monitors relay] sh exit 1: " + gapNotice,
		"[cc-monitors relay] sh exit 1: " + removedNotice,
	}
	if got := receive(t, f, 2); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	waitRemoved(t, s)
}

func TestRetriesDelivery(t *testing.T) {
	f := &fakeSessions{delivered: make(chan string, 10), failures: 2}
	s := newTestServer(t, f)
	startServer(t, s)

	add(t, s, "true")

	if got := receive(t, f, 1); got[0] != "[cc-monitors relay] sh true: "+finishNotice {
		t.Fatalf("got %q", got)
	}
}

func TestRestoresRelaysWithGapNotice(t *testing.T) {
	f := &fakeSessions{delivered: make(chan string, 10)}
	s := newTestServer(t, f)
	r := Relay{ID: "r1", Session: Session{ID: "abc-full"}, Monitor: "sh", Args: []string{"sleep 60"}, Target: []string{"sleep 60"}}
	if err := saveState(s.StatePath, []Relay{r}); err != nil {
		t.Fatal(err)
	}
	startServer(t, s)

	if got := receive(t, f, 1); got[0] != "[cc-monitors relay] sh sleep 60: "+gapNotice {
		t.Fatalf("got %q", got)
	}
	if got := s.list(""); len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("list = %v", got)
	}
}

func TestSocketRoundTrip(t *testing.T) {
	dir, err := os.MkdirTemp("", "relay")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "relay.sock")

	if _, err := Call(socket, Request{Op: "ls"}); err == nil {
		t.Fatal("Call succeeded with no relay process")
	}

	s := newTestServer(t, &fakeSessions{})
	ln, err := Listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- s.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	if _, err := Listen(socket); err == nil {
		t.Fatal("second Listen succeeded")
	}
	resp, err := Call(socket, Request{Op: "add", Session: "abc", Monitor: "sh", Args: []string{"sleep 60"}})
	if err != nil {
		t.Fatal(err)
	}
	ls, err := Call(socket, Request{Op: "ls"})
	if err != nil || len(ls.Relays) != 1 || ls.Relays[0].ID != resp.ID {
		t.Fatalf("ls = %+v, %v", ls, err)
	}
	if _, err := Call(socket, Request{Op: "rm", ID: "nope"}); err == nil {
		t.Fatal("rm of unknown relay succeeded")
	}
}
