package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Request is one JSON line sent to the relay process.
type Request struct {
	Op      string   `json:"op"` // add, rm or ls
	Session string   `json:"session,omitempty"`
	Monitor string   `json:"monitor,omitempty"`
	Args    []string `json:"args,omitempty"`
	Cwd     string   `json:"cwd,omitempty"`
	ID      string   `json:"id,omitempty"`
}

// Response is one JSON line sent back by the relay process.
type Response struct {
	Error  string  `json:"error,omitempty"`
	ID     string  `json:"id,omitempty"`
	Relays []Relay `json:"relays,omitempty"`
}

// Call sends req to the relay process listening on socket.
func Call(socket string, req Request) (Response, error) {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return Response{}, fmt.Errorf("relay serve is not running: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}
	if resp.Error != "" {
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

// Listen opens the relay socket, readable and writable only by the user.
func Listen(socket string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return nil, err
	}
	if conn, err := net.DialTimeout("unix", socket, time.Second); err == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("relay serve is already running on %s", socket)
	}
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	return ln, nil
}
