package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yokonao/cc-monitors/internal/monitor"
	"github.com/yokonao/cc-monitors/internal/prcheck"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cc-monitors",
		Short: "Monitors for Claude Code's Monitor tool",
	}
	root.AddCommand(newPRCICmd())
	return root
}

func newPRCICmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "pr-ci <pr | url | branch>",
		Short: "Watch a PR's CI checks, emitting check_failed / checks_passed events",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if interval <= 0 {
				return fmt.Errorf("--interval must be positive")
			}

			engine := &monitor.Engine{
				Prober:   prcheck.NewChecker(args[0], interval),
				Interval: interval,
				Out:      os.Stdout,
				Err:      os.Stderr,
			}
			if code := engine.Run(); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", prcheck.DefaultInterval, "duration between polls")

	return cmd
}
