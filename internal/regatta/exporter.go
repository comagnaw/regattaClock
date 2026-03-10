package regatta

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/comagnaw/regattaClock/internal/exporter"
)

// exporter - export race images using directory selection dialog
func (r *Regatta) exporter() {
	dialog.ShowFolderOpen(r.exportCallback(), r.window)
}

// exportCallback - function used as callback for directory selection
func (r *Regatta) exportCallback() func(fyne.ListableURI, error) {
	return func(dir fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}

		if dir == nil {
			return
		}

		outputDir := dir.Path()
		exporter.Export(*r.RegattaData, outputDir)

		dialog.ShowInformation("Export", "Successfully exported race images", r.window)
	}
}