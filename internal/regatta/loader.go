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

		r.debugLoader()
		applog.Info("regatta parsed", "component", "loader",
			"name", r.RegattaData.Name, "races", r.RegattaData.ScheduledRaces())
		r.confirmImportedRegatta(fromStartup)
	}
}

// confirmImportedRegatta asks the Regatta Director to confirm the metadata of
// the just-parsed workbook before the schedule is written. Denying returns to
// file selection, so a wrong workbook never lands.
func (r *Regatta) confirmImportedRegatta(fromStartup bool) {
	dialog.ShowConfirm(
		common.ConfirmRegattaTitle,
		fmt.Sprintf(common.ConfirmImportedRegattaMessage,
			r.RegattaData.Name, r.RegattaData.Date, r.RegattaData.ScheduledRaces()),
		func(yes bool) {
			if !yes {
				r.loader(fromStartup)
				return
			}
			r.applyImportedRegatta()
		},
		r.window,
	)
}

// applyImportedRegatta runs the RegattaKey guard (persona-plan.md 3b), writes
// the schedule when it is clear to do so, and enters/refreshes the director
// tree. A blocked import shows a dialog and leaves the tree untouched.
func (r *Regatta) applyImportedRegatta() {
	r.guardScheduleWrite(func() {
		applog.Info("regatta imported", "component", "loader",
			"name", r.RegattaData.Name, "races", r.RegattaData.ScheduledRaces())
		r.startDirectorFlow()
	})
}

// reloadSchedule re-reads the workbook the current schedule was imported from
// and runs it back through the confirm + guard path. Used by the Reload
// Schedule menu item (persona-plan.md 3b: a manual reload runs the same checks).
func (r *Regatta) reloadSchedule() {
	uri := r.RegattaData.URI
	if uri == common.EmptyString {
		dialog.ShowInformation(common.ReloadScheduleTitle, common.NoOriginRecordedMessage, r.window)
		return
	}
	before := scheduleFromRegattaData(r.RegattaData).ContentHash()
	if err := r.setRegattaData(uri); err != nil {
		applog.Error("schedule reload failed", "component", "loader", "path", uri, "err", err)
		dialog.ShowError(fmt.Errorf("%s: %w", common.ReloadFailedMessage, err), r.window)
		return
	}
	r.debugLoader()

	// persona-plan.md 3b step 4/7: a reload that does not change the schedule
	// content must not rewrite regattaSchedule.json.
	if scheduleFromRegattaData(r.RegattaData).ContentHash() == before {
		applog.Info("reload: workbook has not changed the schedule", "component", "loader")
		dialog.ShowInformation(common.ReloadScheduleTitle, common.OriginUnchangedMessage, r.window)
		return
	}
	r.confirmImportedRegatta(false)
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
