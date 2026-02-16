package tray

import "context"

type App interface {
	Run(ctx context.Context) error
}

type Noop struct{}

func NewNoop() App { return Noop{} }

func (Noop) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
