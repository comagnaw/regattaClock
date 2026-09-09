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
		r.confirmImportedRegatta(fromStartup, r.markExcelStepDone)
	}
}

// confirmImportedRegatta asks the Regatta Director to confirm the metadata of
// the just-parsed workbook before it is acted on. Denying returns to file
// selection, so a wrong workbook never lands. onConfirm is what a Yes runs:
// markExcelStepDone from the setup view (advance the stepper, no write yet), or
// applyImportedRegatta from a Reload Schedule (write straight through).
func (r *Regatta) confirmImportedRegatta(fromStartup bool, onConfirm func()) {
	dialog.ShowConfirm(
		common.ConfirmRegattaTitle,
		fmt.Sprintf(common.ConfirmImportedRegattaMessage,
			r.RegattaData.Name, r.RegattaData.Date, r.RegattaData.ScheduledRaces()),
		func(yes bool) {
			if !yes {
				r.loader(fromStartup)
				return
			}
			onConfirm()
		},
		r.window,
	)
}

// markExcelStepDone records that the setup view's Excel step is satisfied - the
// workbook is parsed into r.RegattaData and the Regatta Director has confirmed
// it - and re-renders the setup view so the check mark, parsed summary and file
// path appear and Start Regatta ungates once the save folder is also set. It
// writes nothing to disk; the schedule is written only when Start Regatta runs
// applyImportedRegatta.
func (r *Regatta) markExcelStepDone() {
	r.loadState.excelLoaded = true
	applog.Info("regatta workbook confirmed", "component", "setup",
		"name", r.RegattaData.Name, "races", r.RegattaData.ScheduledRaces())
	r.showDirectorSetup()
}

// applyImportedRegatta runs the RegattaKey guard (persona-plan.md 3b), writes
// the schedule when it is clear to do so, and enters/refreshes the director
// tree. A blocked import shows a dialog and leaves the tree untouched. Reached
// from Start Regatta on the setup view (which only enables once a save folder is
// set) and from Reload Schedule (always from the tree, folder always set); the
// directorSession check is defence in depth against guardScheduleWrite's silent
// no-op when no folder is configured.
func (r *Regatta) applyImportedRegatta() {
	if _, ok := r.directorSession(); !ok {
		dialog.ShowInformation(common.SetRegattaDirTitle, common.SetupNeedSaveDirMessage, r.window)
		return
	}
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
	r.confirmImportedRegatta(false, r.applyImportedRegatta)
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
