package regatta

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// loader - load excel file using dialog. The filter keeps this browser visibly
// distinct from the directory browser behind the config Change button, which
// otherwise looks identical.
func (r *Regatta) loader(fromStartup bool) {
	fileDialog := dialog.NewFileOpen(r.callback(fromStartup), r.window)
	fileDialog.SetFilter(storage.NewExtensionFileFilter(common.RegattaFileExtensions))

	// Open in the configured regatta directory, the likeliest home of the
	// spreadsheet, rather than wherever the last dialog happened to be.
	if regattaDir := r.App.Preferences().String(common.PrefRegattaDir); regattaDir != common.EmptyString {
		if location, err := storage.ListerForURI(storage.NewFileURI(regattaDir)); err == nil {
			fileDialog.SetLocation(location)
		}
	}

	fileDialog.Show()
}

// callback - function used as callback on loader.
func (r *Regatta) callback(fromStartup bool) func(fyne.URIReadCloser, error) {
	return func(fileReader fyne.URIReadCloser, err error) {

		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}

		if fileReader == nil {
			// User cancelled, show reminder
			if fromStartup {
				dialog.ShowInformation(
					"Load Later",
					"You can load the Excel file later by selecting 'Import Regatta Table' from the menu.",
					r.window,
				)
			}
			return
		}
		defer fileReader.Close()

		filePath, err := getFilePath(fileReader)
		if err != nil {
			applog.Error("regatta import failed", "component", "loader", "err", err)
			dialog.ShowError(err, r.window)
			return
		}

		if err = r.setRegattaData(filePath); err != nil {
			applog.Error("regatta import failed", "component", "loader", "path", filePath, "err", err)
			dialog.ShowError(err, r.window)
			return
		}

		r.saveRegattaData()

		applog.Info("regatta imported", "component", "loader",
			"name", r.RegattaData.Name, "races", r.RegattaData.ScheduledRaces())
		r.debugLoader()
		r.refreshContent()

		dialog.ShowInformation("Import", "Successfully read Excel file", r.window)

		r.showRaceTree()
	}
}

func getFilePath(fileReader fyne.URIReadCloser) (string, error) {
	uri := fileReader.URI()
	if uri.Extension() != ".xlsx" && uri.Extension() != ".xlsm" {
		return common.EmptyString, fmt.Errorf("only .xlsx files are supported")
	}

	return uri.Path(), nil
}

func (r *Regatta) setRegattaData(filePath string) error {
	regattaData, err := reader.ReadExcelFile(filePath)
	if err != nil {
		return err
	}

	r.RegattaData = regattaData
	return nil
}

// debugLoader - verbose dump of what was just parsed, emitted only when the
// Debug preference is on (applog drops it otherwise).
func (r *Regatta) debugLoader() {
	applog.Debug("regatta data parsed", "component", "loader",
		"races_total", len(r.RegattaData.Races),
		"races_scheduled", r.RegattaData.ScheduledRaces(),
		"name", r.RegattaData.Name,
		"date", r.RegattaData.Date,
		"source", fmt.Sprintf("%v", r.RegattaData.SourceInfo),
	)
}
