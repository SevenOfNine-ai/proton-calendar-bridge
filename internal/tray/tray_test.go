package tray

import (
	"context"
	"testing"
	"time"
)

func TestNoopTray(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	if err := NewNoop().Run(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestFactory(t *testing.T) {
	if New("x", nil) == nil {
		t.Fatal("expected tray app")
	}
}
