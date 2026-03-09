package main

import (
	"os"
	"testing"
)

func TestRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if code := run([]string{"help"}); code != 1 {
		t.Fatalf("expected failure when HOME is missing")
	}

	t.Setenv("HOME", t.TempDir())
	if code := run([]string{"help"}); code != 0 {
		t.Fatalf("expected help to succeed")
	}
}

func TestEntrypointMain(t *testing.T) {
	origExit := exitFn
	origArgs := os.Args
	t.Cleanup(func() {
		exitFn = origExit
		os.Args = origArgs
	})

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	captured := -1
	exitFn = func(code int) {
		captured = code
	}
	os.Args = []string{"grafana", "help"}

	main()
	if captured != 0 {
		t.Fatalf("expected exit code 0, got %d", captured)
	}
}
