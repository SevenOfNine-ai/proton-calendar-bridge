//go:build systray

package tray

func New(title string, quit func()) App {
	return NewSystray(title, quit)
}
