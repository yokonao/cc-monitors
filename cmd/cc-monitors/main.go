package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yokonao/cc-monitors/internal/monitor"
	"github.com/yokonao/cc-monitors/internal/prcheck"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "pr-ci":
		os.Exit(runPRCI(os.Args[2:]))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cc-monitors <command> [args]")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  pr-ci <pr | url | branch> [--interval SECONDS]")
}

func runPRCI(args []string) int {
	fs := flag.NewFlagSet("pr-ci", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: cc-monitors pr-ci <pr | url | branch> [--interval SECONDS]")
		fs.PrintDefaults()
	}
	interval := fs.Int("interval", int(prcheck.DefaultInterval/time.Second), "seconds between polls")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "--interval must be positive")
		return 2
	}

	pr := fs.Arg(0)
	if pr == "" {
		fs.Usage()
		return 2
	}

	engine := &monitor.Engine[[]prcheck.Check]{
		Prober:           prcheck.NewChecker(pr),
		Interval:         time.Duration(*interval) * time.Second,
		MaxFetchFailures: prcheck.MaxFetchFailures,
		Out:              os.Stdout,
		Err:              os.Stderr,
	}
	return engine.Run()
}
