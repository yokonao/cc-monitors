package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yokonao/cc-monitors/internal/monitor"
	"github.com/yokonao/cc-monitors/internal/prcheck"
)

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
