package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yokonao/cc-monitors/internal/monitor"
	"github.com/yokonao/cc-monitors/internal/prcheck"
	"github.com/yokonao/cc-monitors/internal/prcomment"
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

func newPRCICmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "pr-ci <pr | url | branch>",
		Short: "Watch a PR's CI checks, emitting check_failed / checks_passed events",
		Args:  positiveInterval(&interval),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := &monitor.Engine{
				Prober:   prcheck.NewChecker(args[0], interval),
				Interval: interval,
				// Generous backstop against a hung gh process: covers every
				// retry attempt plus its interval wait, so it shouldn't fire
				// during normal give-up behavior.
				FetchTimeout: prcheck.MaxFetchFailures * (interval + 2*time.Minute),
				Out:          os.Stdout,
				Err:          os.Stderr,
			}
			code, err := engine.Run(cmd.Context())
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", prcheck.DefaultInterval, "duration between polls")

	return cmd
}

func newPRCommentsCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "pr-comments <pr | url | branch>",
		Short: "Watch a PR for new comments and reviews until it is merged or closed",
		Args:  positiveInterval(&interval),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := &monitor.Engine{
				Prober:       prcomment.NewWatcher(args[0], interval),
				Interval:     interval,
				FetchTimeout: 2 * prcomment.MaxFetchFailures * (interval + 2*time.Minute),
				Out:          os.Stdout,
				Err:          os.Stderr,
			}
			code, err := engine.Run(cmd.Context())
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", prcomment.DefaultInterval, "duration between polls")

	return cmd
}
