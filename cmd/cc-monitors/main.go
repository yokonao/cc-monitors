package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "cc-monitors",
		Short:   "Monitors for Claude Code, run under the Monitor tool or relayed to sessions",
		Version: version,
	}
	root.AddCommand(newPRCICmd(), newPRCommentsCmd(), newRelayCmd())
	return root
}

// monitors are the subcommands relay can run. Each call builds a fresh
// command, so flags can be parsed for validation.
var monitors = map[string]func() *cobra.Command{
	"pr-ci":       newPRCICmd,
	"pr-comments": newPRCommentsCmd,
}

// validateMonitor checks a monitor command line against the monitor's own
// flag definitions and returns its positional arguments.
func validateMonitor(name string, args []string) ([]string, error) {
	newCmd, ok := monitors[name]
	if !ok {
		return nil, fmt.Errorf("unknown monitor %q", name)
	}
	cmd := newCmd()
	if err := cmd.ParseFlags(args); err != nil {
		return nil, err
	}
	target := cmd.Flags().Args()
	if err := cmd.ValidateArgs(target); err != nil {
		return nil, err
	}
	return target, nil
}

func positiveInterval(interval *time.Duration) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if *interval <= 0 {
			return fmt.Errorf("--interval must be positive")
		}
		return cobra.ExactArgs(1)(cmd, args)
	}
}
