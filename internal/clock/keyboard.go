package clock

import (
	"fyne.io/fyne/v2"
)

// keyboardHandler - used to build keyboard shortcuts
type keyboardHandler struct {
	startFunc func()
	lapFunc   func()
}

// Typedkey - map F2 to start button and F4 to lap button
func (k *keyboardHandler) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyF2:
		k.startFunc()
	case fyne.KeyF4:
		k.lapFunc()
	}
}

// setupKeyboardHandler - return function that can be used to handle
// keyboard events.
func (c *Clock) setupKeyboardHandler() func(*fyne.KeyEvent) {
	handler := &keyboardHandler{
		startFunc: c.startFunc(),
		lapFunc:   c.lapFunc(),
	}
	return handler.TypedKey
}
