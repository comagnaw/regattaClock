package regatta

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// raceRow holds the widgets for one race in the timer tree. Only the fields the
// row's role uses are non-nil.
type raceRow struct {
	raceNumber int
	root       *fyne.Container

	title     *widget.Label
	startTime *widget.Label // start timer + finish timer
	progress  *widget.Label // finish timer

	startBtn   *widget.Button // start timer
	clearBtn   *widget.Button // start timer
	restoreBtn *widget.Button // start timer
	timeBtn    *widget.Button // finish timer
}

// timerRaceList builds the role-aware race list and records a raceRow per race
// so later updates can touch a single row.
func (r *Regatta) timerRaceList() *container.Scroll {
	r.rows = make(map[int]*raceRow)
	list := container.NewVBox()

	for _, race := range r.RegattaData.SortedRaces() {
		if !race.HasBoats() {
			continue
		}
		row := r.newRaceRow(race)
		r.rows[race.RaceNumber] = row
		list.Add(row.root)
		r.refreshRow(race.RaceNumber)
	}

	scroll := container.NewScroll(list)
	scroll.SetMinSize(fyne.NewSize(0, raceListMinHeight))
	return scroll
}

func (r *Regatta) newRaceRow(race reader.RaceData) *raceRow {
	n := race.RaceNumber
	row := &raceRow{raceNumber: n, title: widget.NewLabel(race.RaceTitle())}

	switch r.session.Role {
	case persona.RoleStart:
		row.startTime = widget.NewLabel(common.NoStartTimeText)
		row.startBtn = widget.NewButton(common.StartTimeButtonText, func() { r.recordStart(n) })
		row.clearBtn = widget.NewButton(common.ClearButtonText, func() { r.clearStart(n) })
		row.restoreBtn = widget.NewButton(common.RestoreButtonText, func() { r.restoreStart(n) })
		row.root = container.NewHBox(
			row.title, layout.NewSpacer(),
			row.startTime, row.startBtn, row.clearBtn, row.restoreBtn,
		)

	case persona.RoleFinish:
		row.startTime = widget.NewLabel(common.WaitingForStartText)
		row.progress = widget.NewLabel("")
		row.timeBtn = widget.NewButton(common.TimeRaceButtonText, func() { r.openClock(n) })
		row.root = container.NewHBox(
			row.title, layout.NewSpacer(),
			row.startTime, row.progress, row.timeBtn,
		)

	default:
		row.root = container.NewHBox(row.title)
	}
	return row
}

// refreshRow re-renders one race row from the current schedule and in-memory
// logs. Safe to call for a race number with no row.
func (r *Regatta) refreshRow(n int) {
	row := r.rows[n]
	if row == nil {
		return
	}
	if race, ok := r.raceByNumber(n); ok {
		row.title.SetText(race.RaceTitle())
	}

	switch r.session.Role {
	case persona.RoleStart:
		r.refreshStartRow(row)
	case persona.RoleFinish:
		r.refreshFinishRow(row)
	}
}

func (r *Regatta) refreshStartRow(row *raceRow) {
	rec := r.startLog.Races[row.raceNumber]

	if rec.StartedAt != nil {
		row.startTime.SetText(rec.Display)
	} else {
		row.startTime.SetText(common.NoStartTimeText)
	}

	setEnabled(row.startBtn, !r.writesBlocked)
	setEnabled(row.clearBtn, rec.StartedAt != nil && !r.writesBlocked)

	if len(rec.Cleared) > 0 && !r.writesBlocked {
		row.restoreBtn.Show()
		row.restoreBtn.Enable()
	} else {
		row.restoreBtn.Hide()
	}
}

func (r *Regatta) refreshFinishRow(row *raceRow) {
	if rec := r.startLog.Races[row.raceNumber]; rec.StartedAt != nil {
		row.startTime.SetText(rec.Display)
	} else {
		row.startTime.SetText(common.WaitingForStartText)
	}

	res, timed := r.finishLog.Races[row.raceNumber]
	switch {
	case timed && res.Approved:
		row.progress.SetText(common.RaceApprovedText)
	case timed && res.WinningTime != common.EmptyString:
		row.progress.SetText(common.RaceSavedText)
	default:
		row.progress.SetText(common.EmptyString)
	}
}

func (r *Regatta) refreshAllRows() {
	for n := range r.rows {
		r.refreshRow(n)
	}
}

func (r *Regatta) raceByNumber(n int) (reader.RaceData, bool) {
	for _, race := range r.RegattaData.Races {
		if race.RaceNumber == n {
			return race, true
		}
	}
	return reader.RaceData{}, false
}

func setEnabled(b *widget.Button, on bool) {
	if b == nil {
		return
	}
	if on {
		b.Enable()
	} else {
		b.Disable()
	}
}
