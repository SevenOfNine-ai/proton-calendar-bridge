package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestLevel(t *testing.T) {
	cases := map[string]slog.Level{"debug": slog.LevelDebug, "warn": slog.LevelWarn, "error": slog.LevelError, "info": slog.LevelInfo, "x": slog.LevelInfo}
	for in, want := range cases {
		if got := level(in); got != want {
			t.Fatalf("level(%q)=%v want %v", in, got, want)
		}
	}
}

func TestRunValidationError(t *testing.T) {
	t.Setenv("PCB_PROVIDER", "ics")
	t.Setenv("PCB_ICS_URL", "")
	t.Setenv("PCB_BEARER_TOKEN", "")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := run(ctx); err == nil {
		t.Fatal("expected config validation error")
	}
}

func TestRunSuccessCancel(t *testing.T) {
	t.Setenv("PCB_PROVIDER", "ics")
	t.Setenv("PCB_ICS_URL", "https://example.test/a.ics")
	t.Setenv("PCB_BEARER_TOKEN", "secret")
	t.Setenv("PCB_REQUIRE_TOKEN", "false")
	t.Setenv("PCB_BIND_ADDRESS", "127.0.0.1:0")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()
	err := run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected run error: %v", err)
	}
}
