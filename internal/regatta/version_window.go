package regatta

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/text"
	"github.com/comagnaw/regattaClock/internal/version"
)

// showVersionWindow opens (or focuses) an independent window listing the
// compiled build attributes. It is not a dialog: the master window has
// SetMaster(), so closing this one never quits the app.
func (r *Regatta) showVersionWindow() {
	if r.versionWindow != nil {
		r.versionWindow.RequestFocus()
		return
	}
	w := r.App.NewWindow(fmt.Sprintf(common.WindowTitleFormat, common.AppTitle, common.VersionTitle))
	w.SetContent(versionContent(w))
	w.Resize(fyne.NewSize(versionWinWidth, versionWinHeight))
	w.CenterOnScreen()
	w.SetOnClosed(func() { r.versionWindow = nil })
	r.versionWindow = w
	w.Show()
}

// versionContent builds the Key: Value grid plus a "View on GitHub" hyperlink to
// the built commit and a Close button.
func versionContent(w fyne.Window) fyne.CanvasObject {
	info := version.Get()

	rows := make([]fyne.CanvasObject, 0, 16)
	for _, kv := range info.Fields() {
		rows = append(rows, text.BoldLabel(kv[0]+":"), widget.NewLabel(kv[1]))
	}
	src, _ := url.Parse(info.Source) // a nil URL just renders inert - no panic
	rows = append(rows, text.BoldLabel("Source:"), widget.NewHyperlink("View on GitHub", src))

	grid := container.New(layout.NewFormLayout(), rows...)
	closeBtn := widget.NewButton(common.CloseButtonText, w.Close)

	return container.NewBorder(
		container.NewVBox(text.Header2(common.VersionTitle), widget.NewSeparator()),
		container.NewPadded(container.NewCenter(closeBtn)),
		nil, nil,
		container.NewPadded(grid),
	)
}
