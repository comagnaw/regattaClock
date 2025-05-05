package clock

import (
	"fyne.io/fyne/v2"
)

type keyboardHandler struct {
	startFunc func()
	lapFunc   func()
}

func (k *keyboardHandler) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyF2:
		k.startFunc()
	case fyne.KeyF4:
		k.lapFunc()
	}
}

func (c *Clock) setupKeyboardHandler() func(*fyne.KeyEvent) {
	handler := &keyboardHandler{
		startFunc: c.startFunc(),
		lapFunc:   c.lapFunc(),
	}
	return handler.TypedKey
}
