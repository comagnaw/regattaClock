package regatta

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// raceRow holds the widgets for one race row. Every role lays its row out as
// Border(nil,nil,nil,cluster,title) with the title right-aligned, so the race
// column reads straight into the fixed-width columns to its right and the header
// labels line up over them. Only the fields the row's role uses are non-nil.
type raceRow struct {
	raceNumber int
	root       *fyne.Container

	title     *widget.Label
	startTime *widget.Label // start timer, finish timer, director
	progress  *widget.Label // finish timer progress; start timer lock note

	startBtn   *widget.Button // start timer
	clearBtn   *widget.Button // start timer
	restoreBtn *widget.Button // start timer
	timeBtn    *widget.Button // finish timer

	restarts *widget.Label // director
	winTime  *widget.Label // director
	approved *widget.Label // director
}

// raceListBody builds the scrolling race list and records a raceRow per race so
// later updates can touch a single row. The per-role layout lives in
// newRaceRow, so timer and director share this.
func (r *Regatta) raceListBody() *container.Scroll {
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

	// Vertical scroll only: rows are laid out to the window width, so the
	// columns stay visible without scrolling sideways.
	scroll := container.NewVScroll(list)
	scroll.SetMinSize(fyne.NewSize(0, raceListMinHeight))
	return scroll
}

func (r *Regatta) newRaceRow(race reader.RaceData) *raceRow {
	n := race.RaceNumber
	row := &raceRow{raceNumber: n, title: widget.NewLabel(race.RaceTitle())}
	row.title.Alignment = fyne.TextAlignTrailing

	var cluster *fyne.Container
	switch r.session.Role {
	case persona.RoleStart:
		// Start / Clear / Restore, then the collected time, then the lock note.
		// Restore keeps its slot when hidden so the time never shifts.
		row.startTime = widget.NewLabel(common.NoStartTimeText)
		row.startTime.Alignment = fyne.TextAlignTrailing
		row.progress = widget.NewLabel(common.EmptyString) // lock note when the FT is timing this race
		row.startBtn = widget.NewButton(common.StartTimeButtonText, func() { r.recordStart(n) })
		row.clearBtn = widget.NewButton(common.ClearButtonText, func() { r.clearStart(n) })
		row.restoreBtn = widget.NewButton(common.RestoreButtonText, func() { r.restoreStart(n) })
		cluster = container.NewHBox(
			fixedCell(actionsColWidth, container.NewGridWithColumns(3, row.startBtn, row.clearBtn, row.restoreBtn)),
			fixedCell(startTimeColWidth, row.startTime),
			fixedCell(statusColWidth, row.progress),
		)

	case persona.RoleFinish:
		row.startTime = widget.NewLabel(common.WaitingForStartText)
		row.startTime.Alignment = fyne.TextAlignTrailing
		row.progress = widget.NewLabel(common.EmptyString)
		row.timeBtn = widget.NewButton(common.TimeRaceButtonText, func() { r.openClock(n) })
		cluster = container.NewHBox(
			fixedCell(startTimeColWidth, row.startTime),
			fixedCell(statusColWidth, row.progress),
			fixedCell(timeRaceColWidth, row.timeBtn),
		)

	default: // RoleDirector - read-only progress, no buttons.
		row.restarts = trailingLabel(common.NoStartTimeText)
		row.startTime = trailingLabel(common.NoStartTimeText)
		row.winTime = trailingLabel(common.NoStartTimeText)
		row.approved = trailingLabel(common.EmptyString)
		cluster = container.NewHBox(
			fixedCell(restartsColWidth, row.restarts),
			fixedCell(startTimeColWidth, row.startTime),
			fixedCell(winTimeColWidth, row.winTime),
			fixedCell(statusColWidth, row.approved),
		)
	}

	row.root = container.NewBorder(nil, nil, nil, cluster, row.title)
	return row
}

func trailingLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Alignment = fyne.TextAlignTrailing
	return l
}

// refreshRow re-renders one race row from the current schedule and in-memory
// logs. Safe to call for a race number with no row.
func (r *Regatta) refreshRow(n int) {
	row := r.rows[n]
	if row == nil {
		return
	}
	if race, ok := r.raceByNumber(n); ok {
		title := race.RaceTitle()
		if _, flagged := r.scheduleConflicts[n]; flagged {
			title = common.ScheduleConflictMark + title
		}
		row.title.SetText(title)
	}

	switch r.session.Role {
	case persona.RoleStart:
		r.refreshStartRow(row)
	case persona.RoleFinish:
		r.refreshFinishRow(row)
	case persona.RoleDirector:
		r.refreshDirectorRow(row)
	}
}

func (r *Regatta) refreshStartRow(row *raceRow) {
	n := row.raceNumber
	rec := r.startLog.Races[n]

	if rec.StartedAt != nil {
		row.startTime.SetText(rec.Display)
	} else {
		row.startTime.SetText(common.NoStartTimeText)
	}

	// Locked once the finish timer has begun this race (persona-plan.md
	// section 9): no changes to the start time while a result is in progress.
	if note, locked := r.finishLockNote(n); locked {
		row.progress.SetText(note)
		setEnabled(row.startBtn, false)
		setEnabled(row.clearBtn, false)
		row.restoreBtn.Hide()
		return
	}
	row.progress.SetText(common.EmptyString)

	// Once a start time exists the button is done: changing it goes through
	// Clear (non-destructive) then Start Time again, not a second click.
	setEnabled(row.startBtn, rec.StartedAt == nil && !r.writesBlocked)
	setEnabled(row.clearBtn, rec.StartedAt != nil && !r.writesBlocked)

	if len(rec.Cleared) > 0 && !r.writesBlocked {
		row.restoreBtn.Show()
		row.restoreBtn.Enable()
	} else {
		row.restoreBtn.Hide()
	}
}

// finishLockNote reports whether the finish timer has begun race n (a
// RaceResult exists in the mirrored finish.json) and the row note to show.
func (r *Regatta) finishLockNote(n int) (string, bool) {
	if r.finishLog == nil {
		return "", false
	}
	res, ok := r.finishLog.Races[n]
	if !ok {
		return "", false
	}
	if res.WinningTime != "" || res.Approved {
		return common.RaceLockedResultsText, true
	}
	return common.RaceLockedTimingText, true
}

// raceLockedByFinish - guard for the ST mutators.
func (r *Regatta) raceLockedByFinish(n int) bool {
	_, locked := r.finishLockNote(n)
	return locked
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

// refreshDirectorRow fills the read-only progress columns. It is nil-safe
// against r.startLog / r.finishLog, which stay nil until the slice that binds
// the Director session and hydrates both teams' timing files; until then every
// cell shows its placeholder.
func (r *Regatta) refreshDirectorRow(row *raceRow) {
	n := row.raceNumber

	restarts, startDisplay := common.NoStartTimeText, common.NoStartTimeText
	if r.startLog != nil {
		if rec, ok := r.startLog.Races[n]; ok {
			restarts = strconv.Itoa(len(rec.Cleared))
			if rec.StartedAt != nil {
				startDisplay = rec.Display
			}
		}
	}
	row.restarts.SetText(restarts)
	row.startTime.SetText(startDisplay)

	winTime, status := common.NoStartTimeText, common.EmptyString
	if r.finishLog != nil {
		if res, ok := r.finishLog.Races[n]; ok {
			if res.WinningTime != common.EmptyString {
				winTime = res.WinningTime
			}
			switch {
			case res.Approved:
				status = common.RaceApprovedText
			case res.WinningTime != common.EmptyString:
				status = common.RaceSavedText
			}
		}
	}
	row.winTime.SetText(winTime)
	row.approved.SetText(status)
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
