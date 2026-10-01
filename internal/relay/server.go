package relay

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	gapNotice     = "watch was interrupted. Check for events during the gap."
	finishNotice  = "watch finished."
	removedNotice = "relay removed after repeated monitor failures."
)

// Sessions finds and wakes Claude Code sessions.
type Sessions interface {
	// Resolve looks up a session by its session ID or short id.
	Resolve(ctx context.Context, session string) (Session, error)
	Deliver(ctx context.Context, s Session, text string) error
}

// Server is the relay process: it accepts registrations over the socket,
// persists them, and runs one worker per relay.
type Server struct {
	StatePath string
	Sessions  Sessions
	// Validate checks a monitor command line and returns its positional
	// arguments (the target).
	Validate func(monitor string, args []string) (target []string, err error)
	// Command builds the child process that runs a relay's monitor.
	Command func(ctx context.Context, r Relay) *exec.Cmd
	Log     *log.Logger

	DeliveryAttempts int
	DeliveryWait     time.Duration
	BackoffMin       time.Duration
	BackoffMax       time.Duration
	MaxFailures      int
	ResetAfter       time.Duration // a run this long resets the failure count

	ctx    context.Context
	wg     sync.WaitGroup
	mu     sync.Mutex
	relays map[string]*entry
}

type entry struct {
	relay  Relay
	cancel context.CancelFunc
}

func NewServer(statePath string, sessions Sessions, validate func(string, []string) ([]string, error)) (*Server, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &Server{
		StatePath: statePath,
		Sessions:  sessions,
		Validate:  validate,
		Command: func(ctx context.Context, r Relay) *exec.Cmd {
			cmd := exec.CommandContext(ctx, exe, append([]string{r.Monitor}, r.Args...)...)
			cmd.Dir = r.Cwd
			return cmd
		},
		Log:              log.New(os.Stderr, "", log.LstdFlags),
		DeliveryAttempts: 30,
		DeliveryWait:     time.Minute,
		BackoffMin:       time.Minute,
		BackoffMax:       30 * time.Minute,
		MaxFailures:      10,
		ResetAfter:       time.Hour,
	}, nil
}

// Serve restores relays from the state file and handles requests on ln until
// ctx is done.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	if err := s.start(ctx); err != nil {
		return err
	}
	defer s.wg.Wait()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.serveConn(conn)
	}
}

// start restores relays, notifying each session of the gap while relay was down.
func (s *Server) start(ctx context.Context) error {
	relays, err := loadState(s.StatePath)
	if err != nil {
		return err
	}
	s.ctx = ctx
	s.relays = map[string]*entry{}
	for _, r := range relays {
		s.relays[r.ID] = s.run(r, true)
	}
	s.Log.Printf("restored %d relays", len(relays))
	return nil
}

func (s *Server) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(time.Minute))

	var req Request
	var resp Response
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		resp.Error = fmt.Sprintf("bad request: %v", err)
	} else {
		resp = s.handle(req)
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

func (s *Server) handle(req Request) Response {
	switch req.Op {
	case "add":
		id, err := s.add(req)
		if err != nil {
			return Response{Error: err.Error()}
		}
		return Response{ID: id}
	case "rm":
		if err := s.remove(req.ID); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{ID: req.ID}
	case "ls":
		return Response{Relays: s.list(req.Session)}
	default:
		return Response{Error: fmt.Sprintf("unknown op %q", req.Op)}
	}
}

// add registers a relay, or updates the monitor flags of an existing relay
// with the same session, monitor and target.
func (s *Server) add(req Request) (string, error) {
	target, err := s.Validate(req.Monitor, req.Args)
	if err != nil {
		return "", err
	}
	sess, err := s.Sessions.Resolve(s.ctx, req.Session)
	if err != nil {
		return "", err
	}
	r := Relay{
		Session: sess,
		Monitor: req.Monitor,
		Args:    req.Args,
		Target:  target,
		Cwd:     req.Cwd,
		Created: time.Now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.relays {
		old := e.relay
		if old.Session.ID != sess.ID || old.Monitor != r.Monitor || !slices.Equal(old.Target, target) {
			continue
		}
		if slices.Equal(old.Args, r.Args) && old.Cwd == r.Cwd {
			return id, nil
		}
		e.cancel()
		r.ID, r.Created = id, old.Created
		s.relays[id] = s.run(r, false)
		s.Log.Printf("relay %s: updated to %s", id, r.CommandLine())
		return id, s.save()
	}

	r.ID = newID()
	for s.relays[r.ID] != nil {
		r.ID = newID()
	}
	s.relays[r.ID] = s.run(r, false)
	s.Log.Printf("relay %s: added %s for session %s", r.ID, r.CommandLine(), sess.ID)
	return r.ID, s.save()
}

func (s *Server) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.relays[id]
	if !ok {
		return fmt.Errorf("no relay %q", id)
	}
	e.cancel()
	delete(s.relays, id)
	s.Log.Printf("relay %s: removed", id)
	return s.save()
}

// drop removes e once its worker is done, unless e was already replaced.
func (s *Server) drop(e *entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.relays[e.relay.ID] != e {
		return
	}
	delete(s.relays, e.relay.ID)
	if err := s.save(); err != nil {
		s.Log.Printf("relay %s: %v", e.relay.ID, err)
	}
}

func (s *Server) list(session string) []Relay {
	s.mu.Lock()
	defer s.mu.Unlock()
	relays := s.snapshot()
	if session != "" {
		relays = slices.DeleteFunc(relays, func(r Relay) bool {
			return r.Session.ID != session && r.Session.ShortID != session
		})
	}
	return relays
}

func (s *Server) snapshot() []Relay {
	relays := []Relay{}
	for _, e := range s.relays {
		relays = append(relays, e.relay)
	}
	slices.SortFunc(relays, func(a, b Relay) int { return a.Created.Compare(b.Created) })
	return relays
}

func (s *Server) save() error {
	if err := saveState(s.StatePath, s.snapshot()); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

// run starts the worker for r. Callers hold s.mu or have no other goroutines.
func (s *Server) run(r Relay, gap bool) *entry {
	ctx, cancel := context.WithCancel(s.ctx)
	e := &entry{relay: r, cancel: cancel}
	s.wg.Go(func() {
		defer cancel()
		s.work(ctx, e, gap)
	})
	return e
}

// work runs the monitor, restarting it with backoff on failure, and delivers
// notices in order without blocking the monitor's output.
func (s *Server) work(ctx context.Context, e *entry, gap bool) {
	r := e.relay
	notices := make(chan string, 64)
	var sender sync.WaitGroup
	sender.Go(func() {
		for n := range notices {
			s.deliver(ctx, r, n)
		}
	})
	defer sender.Wait()
	defer close(notices)

	notify := func(msg string) {
		select {
		case notices <- r.notice(msg):
		case <-ctx.Done():
		}
	}

	if gap {
		notify(gapNotice)
	}
	failures := 0
	for {
		started := time.Now()
		err := s.watch(ctx, r, notify)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			s.Log.Printf("relay %s: watch finished", r.ID)
			notify(finishNotice)
			s.drop(e)
			return
		}

		if time.Since(started) >= s.ResetAfter {
			failures = 0
		}
		failures++
		s.Log.Printf("relay %s: monitor failed (%d/%d): %v", r.ID, failures, s.MaxFailures, err)
		if failures >= s.MaxFailures {
			notify(removedNotice)
			s.drop(e)
			return
		}
		if failures == 1 {
			notify(gapNotice)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(s.backoff(failures)):
		}
	}
}

func (s *Server) backoff(failures int) time.Duration {
	d := s.BackoffMin
	for i := 1; i < failures && d < s.BackoffMax; i++ {
		d *= 2
	}
	return min(d, s.BackoffMax)
}

// watch runs the monitor once, passing each stdout line to notify and logging
// stderr.
func (s *Server) watch(ctx context.Context, r Relay, notify func(string)) error {
	cmd := s.Command(ctx, r)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	var logs sync.WaitGroup
	logs.Go(func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			s.Log.Printf("relay %s: %s", r.ID, sc.Text())
		}
	})
	sc := bufio.NewScanner(stdout)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			notify(line)
		}
	}
	scanErr := sc.Err()
	if scanErr != nil {
		_ = cmd.Process.Kill()
	}
	logs.Wait()
	return errors.Join(cmd.Wait(), scanErr)
}

func (s *Server) deliver(ctx context.Context, r Relay, text string) {
	for attempt := 1; ; attempt++ {
		err := s.Sessions.Deliver(ctx, r.Session, text)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
		s.Log.Printf("relay %s: delivery failed (%d/%d): %v", r.ID, attempt, s.DeliveryAttempts, err)
		if attempt >= s.DeliveryAttempts {
			s.Log.Printf("relay %s: dropped %s", r.ID, text)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.DeliveryWait):
		}
	}
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
