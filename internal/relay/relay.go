// Package relay runs monitors in a long-lived process and relays their events
// to the Claude Code session that registered them.
package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Relay is one registration: run Monitor with Args in Cwd, and deliver each
// event to Session.
type Relay struct {
	ID      string    `json:"id"`
	Session Session   `json:"session"`
	Monitor string    `json:"monitor"`
	Args    []string  `json:"args"`
	Target  []string  `json:"target"` // positional part of Args
	Cwd     string    `json:"cwd"`
	Created time.Time `json:"created"`
}

// Session is a destination Claude Code session as recorded at registration.
type Session struct {
	ID      string `json:"id"`
	ShortID string `json:"short_id"`
	Cwd     string `json:"cwd"` // where to resume it
}

func (r Relay) CommandLine() string {
	return strings.Join(append([]string{r.Monitor}, r.Args...), " ")
}

func (r Relay) notice(msg string) string {
	return fmt.Sprintf("[cc-monitors relay] %s %s: %s", r.Monitor, strings.Join(r.Target, " "), msg)
}

// Dir is where the socket and state file live by default.
func Dir() (string, error) {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "cc-monitors"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "cc-monitors"), nil
}

func loadState(path string) ([]Relay, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var relays []Relay
	if err := json.Unmarshal(b, &relays); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return relays, nil
}

func saveState(path string, relays []Relay) error {
	b, err := json.MarshalIndent(relays, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
