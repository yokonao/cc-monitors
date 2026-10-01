package main

import (
	"slices"
	"testing"
)

func TestValidateMonitor(t *testing.T) {
	target, err := validateMonitor("pr-comments", []string{"--interval", "1m", "https://github.com/o/r/pull/1"})
	if err != nil || !slices.Equal(target, []string{"https://github.com/o/r/pull/1"}) {
		t.Fatalf("target = %q, %v", target, err)
	}

	for _, tc := range []struct {
		monitor string
		args    []string
	}{
		{"nope", []string{"1"}},
		{"relay", []string{"ls"}},
		{"pr-ci", []string{"--bogus", "1"}},
		{"pr-ci", []string{"--interval", "0s", "1"}},
		{"pr-ci", []string{"1", "2"}},
		{"pr-ci", nil},
	} {
		if _, err := validateMonitor(tc.monitor, tc.args); err == nil {
			t.Errorf("validateMonitor(%q, %q) succeeded", tc.monitor, tc.args)
		}
	}
}
