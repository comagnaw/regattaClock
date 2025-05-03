package regattaClock

import (
	"fyne.io/fyne/v2"
)

type keyboardHandler struct {
	startFunc func()
	lapFunc   func()
}

func (h *keyboardHandler) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyF2:
		h.startFunc()
	case fyne.KeyF4:
		h.lapFunc()
	}
}

func (a *App) setupKeyboardHandler() func(*fyne.KeyEvent) {
	handler := &keyboardHandler{
		startFunc: a.startFunc(),
		lapFunc:   a.lapFunc(),
	}
	return handler.TypedKey
}
