//go:build systray

package tray

import (
	"context"

	"github.com/getlantern/systray"
)

type Systray struct {
	Title string
	Quit  func()
}

func NewSystray(title string, quit func()) App {
	return &Systray{Title: title, Quit: quit}
}

func (s *Systray) Run(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		systray.Quit()
	}()
	systray.Run(func() {
		systray.SetTitle(s.Title)
		mQuit := systray.AddMenuItem("Quit", "Quit Proton Calendar Bridge")
		go func() {
			<-mQuit.ClickedCh
			if s.Quit != nil {
				s.Quit()
			}
			systray.Quit()
		}()
	}, func() {
		close(done)
	})
	<-done
	return nil
}
