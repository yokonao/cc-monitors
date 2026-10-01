package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yokonao/cc-monitors/internal/monitor"
	"github.com/yokonao/cc-monitors/internal/prcomment"
)

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
