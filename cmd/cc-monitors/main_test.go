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

	const url = "https://github.com/o/r/pull/1"
	for _, tc := range []struct {
		monitor string
		args    []string
	}{
		{"nope", []string{url}},
		{"relay", []string{"ls"}},
		{"pr-ci", []string{"--bogus", url}},
		{"pr-ci", []string{"--interval", "0s", url}},
		{"pr-ci", []string{url, url}},
		{"pr-ci", nil},
		{"pr-ci", []string{"1"}},
		{"pr-ci", []string{"my-branch"}},
		{"pr-merge", []string{"o/r#1"}},
		{"pr-comments", []string{"https://github.com/o/r/pull/1/files"}},
		{"pr-comments", []string{"https://github.com/o/r/issues/1"}},
		{"pr-comments", []string{"http://github.com/o/r/pull/1"}},
	} {
		if _, err := validateMonitor(tc.monitor, tc.args); err == nil {
			t.Errorf("validateMonitor(%q, %q) succeeded", tc.monitor, tc.args)
		}
	}
}
