package regatta

import (
	"fmt"

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
		defer r.window.RequestFocus()

		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}

		if dir == nil {
			return
		}

		outputDir := dir.Path()
		result := exporter.Export(*r.RegattaData, outputDir)

		if result.HasErrors() {
			msg := fmt.Sprintf("Export completed with errors.\nSucceeded: %d\nFailed: %d",
				result.Succeeded, result.Failed)
			dialog.ShowError(fmt.Errorf("%s", msg), r.window)
			return
		}

		msg := fmt.Sprintf("Successfully exported %d race images", result.Succeeded)
		dialog.ShowInformation("Export", msg, r.window)
	}
}
