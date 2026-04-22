package lifecycle

import (
	"testing"
	"time"
)

func TestEphemeralSessionNameUsesBasenameAndTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 23, 12, 34, 56, 0, time.UTC)
	got := EphemeralSessionName("/home/tester/source/repos/projmux", now)
	want := "projmux-20260423-123456"
	if got != want {
		t.Fatalf("EphemeralSessionName() = %q, want %q", got, want)
	}
}

func TestEphemeralSessionNameSanitizesAndFallsBackToShell(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 23, 12, 34, 56, 0, time.UTC)

	if got, want := EphemeralSessionName("/tmp/hello world", now), "hello-world-20260423-123456"; got != want {
		t.Fatalf("EphemeralSessionName() = %q, want %q", got, want)
	}

	if got, want := EphemeralSessionName("/", now), "shell-20260423-123456"; got != want {
		t.Fatalf("EphemeralSessionName() = %q, want %q", got, want)
	}
}
