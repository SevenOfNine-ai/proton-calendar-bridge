package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/config"
	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/provider"
)

type fakeProvider struct{}

func (fakeProvider) Name() string                                             { return "fake" }
func (fakeProvider) ListCalendars(context.Context) ([]domain.Calendar, error) { return nil, nil }
func (fakeProvider) ListEvents(context.Context, string, time.Time, time.Time) ([]domain.Event, error) {
	return nil, nil
}
func (fakeProvider) CreateEvent(context.Context, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, provider.NotSupportedError{}
}
func (fakeProvider) UpdateEvent(context.Context, string, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, provider.NotSupportedError{}
}
func (fakeProvider) DeleteEvent(context.Context, string) error { return provider.NotSupportedError{} }

func TestApplicationRunCancel(t *testing.T) {
	cfg := config.Config{BindAddress: "127.0.0.1:0", RequireBearerToken: false, RequestTimeout: time.Second}
	a := New(cfg, fakeProvider{}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	if err := a.Run(ctx); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

type errTray struct{}

func (errTray) Run(context.Context) error { return errors.New("tray failed") }

func TestApplicationRunNoListeners(t *testing.T) {
	cfg := config.Config{BindAddress: "", UnixSocketPath: "", RequireBearerToken: false, EnableTray: false}
	a := New(cfg, fakeProvider{}, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := a.Run(ctx); err != nil {
		t.Fatalf("expected nil due to no listeners, got %v", err)
	}
}

func TestApplicationRunTrayError(t *testing.T) {
	cfg := config.Config{BindAddress: "", RequireBearerToken: false, EnableTray: true}
	a := New(cfg, fakeProvider{}, errTray{}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Run(ctx); err == nil {
		t.Fatal("expected tray error")
	}
}

func TestBuildProvider(t *testing.T) {
	ics, err := BuildProvider(config.Config{ProviderType: "ics", ICSURL: "https://example.test/a.ics"})
	if err != nil {
		t.Fatalf("ics provider: %v", err)
	}
	if ics.Name() != "ics" {
		t.Fatalf("unexpected provider: %s", ics.Name())
	}

	proton, err := BuildProvider(config.Config{ProviderType: "proton"})
	if err != nil {
		t.Fatalf("proton provider: %v", err)
	}
	if proton.Name() != "proton" {
		t.Fatalf("unexpected provider: %s", proton.Name())
	}

	if _, err := BuildProvider(config.Config{ProviderType: "unknown"}); err == nil {
		t.Fatal("expected invalid provider error")
	}
}
