package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yokonao/cc-monitors/internal/monitor"
	"github.com/yokonao/cc-monitors/internal/prmerge"
)

func newPRMergeCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "pr-merge <pr | url | branch>",
		Short: "Watch a PR until it is merged or closed, emitting pr_closed",
		Args:  positiveInterval(&interval),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := &monitor.Engine{
				Prober:       prmerge.NewWatcher(args[0], interval),
				Interval:     interval,
				FetchTimeout: prmerge.MaxFetchFailures * (interval + 2*time.Minute),
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
	cmd.Flags().DurationVar(&interval, "interval", prmerge.DefaultInterval, "duration between polls")

	return cmd
}
