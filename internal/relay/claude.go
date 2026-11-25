package relay

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const deliveryTimeout = 5 * time.Minute

//go:embed all:plugin
var pluginFS embed.FS

// maxStdoutInError caps the stdout a failed call quotes in its error.
const maxStdoutInError = 300

// Claude implements Sessions with the claude CLI. Messaging a running session
// relies on a headless session running a plugin on relay's behalf.
type Claude struct {
	// Dir is the cwd of the headless sender, which names it to the receiver.
	Dir string
	// Log receives each distinct stderr line of successful calls once per process.
	Log *log.Logger
}

var loggedStderr sync.Map

// agent mirrors one entry of `claude agents --json`.
type agent struct {
	SessionID string `json:"sessionId"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cwd       string `json:"cwd"`
}

func (c Claude) Resolve(ctx context.Context, session string) (Session, error) {
	agents, err := c.listAgents(ctx, true)
	if err != nil {
		return Session{}, err
	}
	a, err := findSession(agents, session)
	if err != nil {
		return Session{}, err
	}
	return Session{ID: a.SessionID, ShortID: a.ID}, nil
}

// Deliver sends text to the running session s through the slash command of
// the embedded plugin, which calls $.session.send without a model call. A
// session that is not running is an error rather than resumed.
func (c Claude) Deliver(ctx context.Context, s Session, text string) error {
	ctx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "cc-monitors-relay-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	sub, err := fs.Sub(pluginFS, "plugin")
	if err != nil {
		return err
	}
	if err := os.CopyFS(dir, sub); err != nil {
		return err
	}

	// Without the plugin the command reaches the model as a prompt, so it is a
	// cheap model with no settings, MCP servers or permissions, and the reply
	// tells whether the plugin ran. No transcript is kept, so deliveries stay
	// out of --resume.
	out, err := c.claude(ctx, c.Dir, []string{"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1"},
		"-p", "--plugin-dir", dir, "--no-session-persistence", "--setting-sources", "",
		"--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`,
		"--model", "haiku", "--permission-mode", "dontAsk",
		"--", "/send-message "+s.ID+" "+text)
	if err != nil {
		return err
	}
	if reply := strings.TrimSpace(string(out)); !strings.HasPrefix(reply, "cc-monitors-relay: sent to ") {
		return fmt.Errorf("send to %s: %s", s.ID, tail(reply, maxStdoutInError))
	}
	return nil
}

// findSession matches a session ID or short id, then an exact session name,
// then part of a session name, like `claude attach`.
func findSession(agents []agent, session string) (agent, error) {
	lower := strings.ToLower(session)
	for _, match := range []func(agent) bool{
		func(a agent) bool { return a.SessionID == session || a.ID == session },
		func(a agent) bool { return a.Name == session },
		func(a agent) bool { return a.Name != "" && strings.Contains(strings.ToLower(a.Name), lower) },
	} {
		var found []agent
		for _, a := range agents {
			if match(a) && !slices.ContainsFunc(found, func(f agent) bool { return f.SessionID == a.SessionID }) {
				found = append(found, a)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0], nil
		default:
			return agent{}, fmt.Errorf("session %q matches more than one session", session)
		}
	}
	return agent{}, fmt.Errorf("session %q not found in claude agents", session)
}

func (c Claude) listAgents(ctx context.Context, all bool) ([]agent, error) {
	args := []string{"agents", "--json"}
	if all {
		args = append(args, "--all")
	}
	out, err := c.claude(ctx, "", nil, args...)
	if err != nil {
		return nil, err
	}
	var agents []agent
	if err := json.Unmarshal(out, &agents); err != nil {
		return nil, fmt.Errorf("unparseable claude agents output: %w", err)
	}
	return agents, nil
}

func (c Claude) claude(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	var out, errBuf strings.Builder
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	// Without it, a process claude spawned (an MCP server, say) that still
	// holds the output pipes keeps Run blocked after ctx is done.
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errBuf.String()); msg != "" {
			return nil, fmt.Errorf("claude %s: %s", args[0], msg)
		}
		// claude -p in auto mode exits 1 with the reason on stdout when it
		// refused an action, as Claude Code 2.1.319 and later do.
		if msg := strings.TrimSpace(out.String()); msg != "" {
			return nil, fmt.Errorf("claude %s: %w: %s", args[0], err, tail(msg, maxStdoutInError))
		}
		return nil, fmt.Errorf("claude %s: %w", args[0], err)
	}
	if c.Log != nil {
		for line := range strings.Lines(errBuf.String()) {
			line = strings.TrimSpace(line)
			if _, seen := loggedStderr.LoadOrStore(line, true); line != "" && !seen {
				c.Log.Printf("claude %s: %s", args[0], line)
			}
		}
	}
	return []byte(out.String()), nil
}

// tail returns the last n bytes of s, cut at a rune boundary and prefixed with
// "..." if anything was dropped. A final reply puts the reason last.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	i := len(s) - n
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return "..." + s[i:]
}
