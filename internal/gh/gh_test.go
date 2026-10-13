package gh

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunReturnsWhenDescendantHoldsPipes(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 10 &\nsleep 10\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":/bin:/usr/bin")
	waitDelay = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, _, err := Run(ctx); err == nil {
		t.Fatal("want error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run returned after %v, want shortly after ctx is done", elapsed)
	}
}
