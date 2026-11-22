package regatta

import (
	"fmt"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish/destinations"
	"github.com/comagnaw/regattaClock/internal/text"
)

// Publish destinations (publish-destinations.md): the primary finish timer
// chooses which destinations this regatta's results go to, and the folder
// for each that needs one. The choice is recorded in its finish.json
// (store.FinishLog.Publish) - on the PFT because only that machine has the
// destination folders (an OneDrive library, a Google Drive for Desktop
// folder) mapped.

// publishDestinationsItem - the PFT-only menu item opening the dialog.
func (r *Regatta) publishDestinationsItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.PublishDestinationsTitle, r.showPublishDestinations)
}

// publishDraft is the dialog's working copy: nothing is saved until Save.
type publishDraft struct {
	enabled map[string]bool
	dirs    map[string]string
}

// showPublishDestinations opens the destinations dialog: one row per kind - a
// checkbox, and for a folder kind its folder and a Choose Folder button.
// Ticking a folder kind with no folder opens the folder browser. Save checks
// the draft and records it (setPublishSettings); Cancel discards it.
func (r *Regatta) showPublishDestinations() {
	if !r.isPublisher() || r.finishLog == nil {
		dialog.NewInformation(common.PublishDestinationsTitle, common.PublishDestinationsPFTOnlyMessage, r.window).Show()
		return
	}

	draft := publishDraft{enabled: map[string]bool{}, dirs: map[string]string{}}
	for _, id := range r.finishLog.PublishDestinations() {
		draft.enabled[id] = true
	}
	rows := container.NewVBox(text.Wrapping(common.PublishDestinationsIntro))
	for _, k := range destinations.Kinds {
		draft.dirs[k.ID] = r.savedResultsDir(k.ID)
		rows.Add(r.destinationRow(k, &draft))
	}

	saveBtn := widget.NewButton(common.SaveButtonText, nil)
	saveBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton(common.CancelButtonText, nil)
	d := dialog.NewCustomWithoutButtons(common.PublishDestinationsTitle, rows, r.window)
	d.SetButtons([]fyne.CanvasObject{cancelBtn, saveBtn})
	cancelBtn.OnTapped = d.Hide
	saveBtn.OnTapped = func() {
		kinds, problem := draft.validate()
		if problem != common.EmptyString {
			dialog.NewInformation(common.PublishDestinationsTitle, problem, r.window).Show()
			return
		}
		if r.setPublishSettings(kinds, draft.dirs) {
			d.Hide()
		}
	}
	d.Resize(fyne.NewSize(640, 0))
	d.Show()
}

// destinationRow is one kind's row in the dialog. A kind that is not built
// yet is shown disabled, as coming soon - unless a newer build enabled it in
// this finish.json, when it stays tickable so it can be switched off.
func (r *Regatta) destinationRow(k destinations.Kind, draft *publishDraft) fyne.CanvasObject {
	label := k.Label
	if !k.Available {
		label = fmt.Sprintf(common.ComingSoonFormat, k.Label)
	}
	check := widget.NewCheck(label, nil)
	check.SetChecked(draft.enabled[k.ID])
	if !k.Available && !draft.enabled[k.ID] {
		check.Disable()
	}
	if !k.NeedsFolder {
		check.OnChanged = func(on bool) { draft.enabled[k.ID] = on }
		return check
	}

	folder := text.Truncating(folderText(draft.dirs[k.ID]))
	var choose *widget.Button
	choose = widget.NewButton(common.ChooseFolderButtonText, func() {
		r.browseResultsFolder(k.ID, draft.dirs[k.ID], func(dir string) {
			draft.dirs[k.ID] = dir
			folder.SetText(folderText(dir))
			check.SetChecked(true)
		}, nil)
	})
	check.OnChanged = func(on bool) {
		draft.enabled[k.ID] = on
		if on && draft.dirs[k.ID] == common.EmptyString {
			choose.OnTapped()
		}
	}
	return container.NewBorder(nil, nil, check, choose, folder)
}

func folderText(dir string) string {
	if dir == common.EmptyString {
		return common.NoFolderChosenText
	}
	return dir
}

// validate returns the enabled kinds in registry order, or the reason the
// draft cannot be saved: nothing enabled, or a folder kind without a folder.
func (d publishDraft) validate() ([]string, string) {
	var kinds []string
	for _, k := range destinations.Kinds {
		if !d.enabled[k.ID] {
			continue
		}
		if k.NeedsFolder && d.dirs[k.ID] == common.EmptyString {
			return nil, fmt.Sprintf(common.PublishDestinationsNeedsFolderFormat, k.Label)
		}
		kinds = append(kinds, k.ID)
	}
	if len(kinds) == 0 {
		return nil, common.PublishDestinationsNoneMessage
	}
	return kinds, common.EmptyString
}

// setPublishSettings records the enabled kinds and their folders in
// finish.json (and each folder as this machine's last-used one for its kind),
// then reloads every destination's published state - a changed set or folder
// is a different publish history. Folders of kinds switched off stay
// recorded, so switching one back on remembers where it went. Reports whether
// it was recorded; guarded like setResultsDir.
func (r *Regatta) setPublishSettings(kinds []string, dirs map[string]string) bool {
	if !r.isPublisher() || r.finishLog == nil {
		return false
	}
	if r.writesBlocked {
		r.showPublishError(common.PublishSettingsNotRecordedMessage)
		return false
	}
	prevDir, prevPublish := r.finishLog.ResultsDir, r.finishLog.Publish.Clone()
	r.finishLog.SetPublishDestinations(kinds)
	for kind, dir := range dirs {
		if dir != common.EmptyString && dir != r.finishLog.PublishDir(kind) {
			r.finishLog.SetPublishDir(kind, dir)
		}
	}
	if err := store.SaveFinish(r.session, r.finishLog); err != nil {
		r.finishLog.ResultsDir, r.finishLog.Publish = prevDir, prevPublish
		applog.Error("publish destinations not recorded", "component", "publish", "kinds", kinds, "error", err)
		r.showPublishError(common.PublishSettingsNotRecordedMessage)
		return false
	}
	for kind, dir := range dirs {
		if dir != common.EmptyString && slices.Contains(kinds, kind) {
			r.App.Preferences().SetString(resultsDirPref(kind), dir)
		}
	}
	applog.Info("publish destinations set", "component", "publish", "kinds", kinds)
	r.publishedLedgers = nil
	r.refreshAllRows()
	r.loadPublishedLedgers()
	r.refreshPublishSummary()
	return true
}

// publishSummaryRow - the Configuration screen's read-only summary of this
// regatta's destinations and folders, with a button opening the dialog. The
// screen is built once, before any session, so the summary is a binding
// refreshed by refreshPublishSummary.
func (r *Regatta) publishSummaryRow() *fyne.Container {
	r.publishSummary = binding.NewString()
	summary := text.Wrapping(common.EmptyString)
	summary.Bind(r.publishSummary)
	r.refreshPublishSummary()
	return container.NewBorder(nil, nil, nil,
		widget.NewButton(common.PublishDestinationsButtonText, r.showPublishDestinations),
		widget.NewForm(widget.NewFormItem(common.PublishSummaryRowLabel, summary)),
	)
}

// refreshPublishSummary rewrites the Configuration summary from finish.json.
func (r *Regatta) refreshPublishSummary() {
	if r.publishSummary == nil {
		return
	}
	if !r.isPublisher() || r.finishLog == nil {
		_ = r.publishSummary.Set(common.PublishSummaryNotPFT)
		return
	}
	var lines []string
	for _, k := range r.enabledKinds() {
		line := k.Label
		if k.NeedsFolder {
			line = fmt.Sprintf(common.PublishSummaryLineFormat, k.Label, folderText(r.savedResultsDir(k.ID)))
		}
		lines = append(lines, line)
	}
	_ = r.publishSummary.Set(strings.Join(lines, "\n"))
}
