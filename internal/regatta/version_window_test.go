package regatta

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/version"
)

func TestMakeMenuHasVersionItem(t *testing.T) {
	r := directorWithRaces(t, nil)

	var found bool
	for _, it := range r.makeMenu().Items[0].Items {
		if it.Label == common.VersionTitle {
			found = true
			if it.Action == nil {
				t.Fatal("Version menu item has no action")
			}
		}
	}
	if !found {
		t.Errorf("no %q item in the app menu", common.VersionTitle)
	}
}

func TestShowVersionWindowIsSingleton(t *testing.T) {
	r := directorWithRaces(t, nil)

	r.showVersionWindow()
	if r.versionWindow == nil {
		t.Fatal("showVersionWindow did not open a window")
	}
	first := r.versionWindow

	r.showVersionWindow()
	if r.versionWindow != first {
		t.Error("a second Version click opened a duplicate window")
	}

	r.versionWindow.Close()
	if r.versionWindow != nil {
		t.Error("closing the window did not clear r.versionWindow")
	}
}

func TestVersionContentHasLinkAndClose(t *testing.T) {
	w := test.NewTempApp(t).NewWindow("t")
	t.Cleanup(w.Close)

	c := versionContent(w)

	var link *widget.Hyperlink
	var closeBtn *widget.Button
	walk(c, func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Hyperlink:
			link = v
		case *widget.Button:
			closeBtn = v
		}
	})

	if link == nil {
		t.Fatal("versionContent has no hyperlink")
	}
	if link.URL == nil || link.URL.String() != version.Get().Source {
		t.Errorf("link URL = %v, want %q", link.URL, version.Get().Source)
	}
	if closeBtn == nil || closeBtn.Text != common.CloseButtonText {
		t.Errorf("versionContent has no %q button", common.CloseButtonText)
	}
}

func walk(o fyne.CanvasObject, fn func(fyne.CanvasObject)) {
	fn(o)
	if c, ok := o.(*fyne.Container); ok {
		for _, child := range c.Objects {
			walk(child, fn)
		}
	}
}
