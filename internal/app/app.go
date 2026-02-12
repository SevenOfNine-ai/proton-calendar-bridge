package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/sevenofnine/proton-calendar-bridge/internal/api"
	"github.com/sevenofnine/proton-calendar-bridge/internal/config"
	"github.com/sevenofnine/proton-calendar-bridge/internal/provider"
	"github.com/sevenofnine/proton-calendar-bridge/internal/security"
	"github.com/sevenofnine/proton-calendar-bridge/internal/tray"
)

type Application struct {
	cfg      config.Config
	provider provider.CalendarProvider
	tray     tray.App
	logger   *slog.Logger
}

func New(cfg config.Config, p provider.CalendarProvider, tr tray.App, logger *slog.Logger) *Application {
	if logger == nil {
		logger = slog.Default()
	}
	if tr == nil {
		tr = tray.NewNoop()
	}
	return &Application{cfg: cfg, provider: p, tray: tr, logger: logger}
}

func (a *Application) Run(ctx context.Context) error {
	server := api.New(api.Options{
		Provider: a.provider,
		Auth: security.BearerAuth{
			Enabled: a.cfg.RequireBearerToken,
			Token:   a.cfg.BearerToken,
		},
		Logger: a.logger,
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 3)
	wg := sync.WaitGroup{}

	if a.cfg.BindAddress != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := server.ServeTCP(ctx, a.cfg.BindAddress); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("tcp server: %w", err)
			}
		}()
	}
	if a.cfg.UnixSocketPath != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := server.ServeUnix(ctx, a.cfg.UnixSocketPath); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("unix server: %w", err)
			}
		}()
	}

	if a.cfg.EnableTray {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.tray.Run(ctx); err != nil {
				errCh <- fmt.Errorf("tray: %w", err)
			}
		}()
	}

	select {
	case err := <-errCh:
		cancel()
		wg.Wait()
		return err
	case <-ctx.Done():
		wg.Wait()
		return nil
	}
}
