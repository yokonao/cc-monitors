package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestValidateExec(t *testing.T) {
	for _, args := range [][]string{
		{"--", "/bin/watch", "--since", "1h"},
		{"/bin/watch", "--since", "1h"},
	} {
		target, err := validateMonitor("exec", args)
		if err != nil || !slices.Equal(target, []string{"/bin/watch", "--since", "1h"}) {
			t.Errorf("validateMonitor(exec, %q) = %q, %v", args, target, err)
		}
	}
	if _, err := validateMonitor("exec", []string{"--"}); err == nil {
		t.Error("exec without a command was accepted")
	}
}

func TestCheckExecAllowed(t *testing.T) {
	allow := filepath.Join(t.TempDir(), "exec.allow")
	if err := checkExecAllowed(allow, "/bin/watch"); err == nil {
		t.Fatal("accepted without an allowlist file")
	}
	if err := os.WriteFile(allow, []byte("# watchers\n\n  /bin/watch  \n/opt/x/../y/tool\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"/bin/watch", "/opt/y/tool"} {
		if err := checkExecAllowed(allow, c); err != nil {
			t.Errorf("%s: %v", c, err)
		}
	}
	for _, c := range []string{"watch", "/bin/other", "/bin/watch2", "# watchers"} {
		if err := checkExecAllowed(allow, c); err == nil {
			t.Errorf("%s was accepted", c)
		}
	}
}
