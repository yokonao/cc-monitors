package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/yokonao/cc-monitors/internal/relay"
)

func newRelayCmd() *cobra.Command {
	var socket string

	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Relay monitor events to Claude Code sessions (experimental)",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			if socket != "" {
				return nil
			}
			dir, err := relay.Dir()
			socket = filepath.Join(dir, "relay.sock")
			return err
		},
	}
	cmd.PersistentFlags().StringVar(&socket, "socket", "", "relay socket path (default $XDG_STATE_HOME/cc-monitors/relay.sock)")
	cmd.AddCommand(
		newRelayServeCmd(&socket),
		newRelayAddCmd(&socket),
		newRelayRmCmd(&socket),
		newRelayLsCmd(&socket),
	)
	return cmd
}

func newRelayServeCmd(socket *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the relay process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := relay.Dir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			s, err := relay.NewServer(filepath.Join(dir, "relays.json"), relay.Claude{Dir: dir}, validateMonitor)
			if err != nil {
				return err
			}
			ln, err := relay.Listen(*socket)
			if err != nil {
				return err
			}
			s.Log.Printf("listening on %s", *socket)
			return s.Serve(cmd.Context(), ln)
		},
	}
}

func newRelayAddCmd(socket *string) *cobra.Command {
	var session string

	cmd := &cobra.Command{
		Use:   "add [--session <id>] <monitor> <target> [monitor flags]",
		Short: "Relay a monitor's events to a session and print the relay ID",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dest, err := sessionOrCaller(session)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			resp, err := relay.Call(*socket, relay.Request{
				Op:      "add",
				Session: dest,
				Monitor: args[0],
				Args:    args[1:],
				Cwd:     cwd,
			})
			if err != nil {
				return err
			}
			fmt.Println(resp.ID)
			return nil
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&session, "session", "", "destination session ID or short id (default $CLAUDE_CODE_SESSION_ID)")
	return cmd
}

func newRelayRmCmd(socket *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <relay-id>",
		Short: "Remove a relay",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, err := relay.Call(*socket, relay.Request{Op: "rm", ID: args[0]})
			return err
		},
	}
}

func newRelayLsCmd(socket *string) *cobra.Command {
	var session string

	cmd := &cobra.Command{
		Use:   "ls [--session <id>]",
		Short: "List relays",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := relay.Call(*socket, relay.Request{Op: "ls", Session: session})
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tSESSION\tCOMMAND\tCREATED")
			for _, r := range resp.Relays {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.ID, r.Session.ID, r.CommandLine(), r.Created.Local().Format(time.DateTime))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "only relays to this session ID or short id")
	return cmd
}

func sessionOrCaller(session string) (string, error) {
	if session != "" {
		return session, nil
	}
	if id := os.Getenv("CLAUDE_CODE_SESSION_ID"); id != "" {
		return id, nil
	}
	return "", errors.New("--session is required outside a Claude Code session")
}
