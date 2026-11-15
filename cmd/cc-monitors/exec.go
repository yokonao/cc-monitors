package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yokonao/cc-monitors/internal/execmon"
)

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [--] <command> [args...]",
		Short: "Run a command and forward the {\"event\", \"data\"} lines it prints",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := execmon.Run(cmd.Context(), args[0], args[1:], os.Stdout, os.Stderr)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// execAllowPath is the allowlist of commands `relay add exec` may run. Only
// `relay serve` reads it, since it runs commands outside the caller's sandbox.
func execAllowPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "cc-monitors", "exec.allow"), nil
}

// checkExecAllowed accepts an absolute command path listed in the allowlist
// file, one path per line, with blank lines and # comments ignored.
func checkExecAllowed(allowPath, command string) error {
	if !filepath.IsAbs(command) {
		return fmt.Errorf("relay exec needs an absolute command path, got %q", command)
	}
	f, err := os.Open(allowPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s is not allowed: %s does not exist", command, allowPath)
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") && filepath.Clean(line) == filepath.Clean(command) {
			return nil
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%s is not allowed: add it to %s", command, allowPath)
}
