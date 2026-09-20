package regatta

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/text"
)

// raceRow holds the widgets for one race row. Every role lays its row out as
// Border(nil,nil,nil,cluster,title) with the title right-aligned, so the race
// column reads straight into the fixed-width columns to its right and the header
// labels line up over them. Every role shows the same data columns
// (scheduledTime, restarts, startTime, winTime, progress) - the "pane of
// glass" race tree (race-state-machine.md); only the action buttons differ
// per role, and only those fields are ever nil.
type raceRow struct {
	raceNumber int
	root       *fyne.Container

	raceNum       *widget.Label
	title         *widget.Label
	scheduledTime *widget.Label
	boatCount     *widget.Label
	restarts      *widget.Label
	startTime     *widget.Label
	winTime       *widget.Label
	progress      *widget.Label

	startBtn   *widget.Button // start timer
	clearBtn   *widget.Button // start timer
	restoreBtn *widget.Button // start timer
	timeBtn    *widget.Button // finish timer
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

// newRaceRow builds one race's row: Race number, then Scheduled Time
// (left-anchored, fixed width) - Title (flexible width, fills the middle,
// RaceDetail() rather than RaceTitle() since the race number already has
// its own column) - the role's action, then Restarts, Start Time, Winning
// Time, Status (right-anchored). Every role shows the same data columns in
// the same order; only the action cell differs per role (Start/Clear/Restore
// for the ST, Time Race for the FT, none for the read-only RD). This is the
// "pane of glass" race tree (race-state-machine.md): one shared row shape,
// a per-role action, not three independent layouts.
func (r *Regatta) newRaceRow(race reader.RaceData) *raceRow {
	n := race.RaceNumber
	row := &raceRow{raceNumber: n, title: widget.NewLabel(race.RaceDetail())}
	row.title.Alignment = fyne.TextAlignTrailing
	row.raceNum = text.TruncatingCenter(strconv.Itoa(n))
	row.scheduledTime = text.TruncatingCenter(race.ScheduledTimeDisplay())
	row.boatCount = text.TruncatingCenter(strconv.Itoa(race.BoatCount))
	row.restarts = text.TruncatingCenter(common.NoStartTimeText)
	row.startTime = text.TruncatingCenter(common.NoStartTimeText)
	row.winTime = text.TruncatingCenter(common.NoStartTimeText)
	row.progress = text.TruncatingCenter(common.EmptyString)

	var action fyne.CanvasObject
	switch r.session.Role {
	case persona.RoleStart:
		// Restore keeps its slot when hidden so the row never shifts.
		row.startBtn = widget.NewButton(common.StartTimeButtonText, func() { r.recordStart(n) })
		row.clearBtn = widget.NewButton(common.ClearButtonText, func() { r.clearStart(n) })
		row.restoreBtn = widget.NewButton(common.RestoreButtonText, func() { r.restoreStart(n) })
		action = fixedCell(actionsColWidth, container.NewGridWithColumns(3, row.startBtn, row.clearBtn, row.restoreBtn))

	case persona.RoleFinish:
		// Matches refreshFinishRow's own "nothing collected yet" default, so
		// there is no flash of different text between construction and the
		// first refresh (raceListBody calls refreshRow immediately after).
		row.startTime.SetText(common.WaitingForStartText)
		row.timeBtn = widget.NewButton(common.TimeRaceButtonText, func() { r.openClock(n) })
		action = fixedCell(timeRaceColWidth, row.timeBtn)
	}

	var cells []fyne.CanvasObject
	if action != nil {
		cells = append(cells, action)
	}
	cells = append(cells,
		fixedCell(restartsColWidth, row.restarts),
		fixedCell(startTimeColWidth, row.startTime),
		fixedCell(winTimeColWidth, row.winTime),
		fixedCell(statusColWidth, row.progress),
	)

	leading := container.NewHBox(
		fixedCell(raceNumColWidth, row.raceNum),
		fixedCell(scheduledTimeColWidth, row.scheduledTime),
		fixedCell(boatCountColWidth, row.boatCount),
	)
	row.root = container.NewBorder(nil, nil, leading, container.NewHBox(cells...), row.title)
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
		title := race.RaceDetail()
		if r.staleLaneMap(n, race) {
			title = common.StaleLaneMapMark + title
		}
		if _, flagged := r.scheduleConflicts[n]; flagged {
			title = common.ScheduleConflictMark + title
		}
		row.title.SetText(title)
		row.scheduledTime.SetText(race.ScheduledTimeDisplay())
		row.boatCount.SetText(strconv.Itoa(race.BoatCount))
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
		// Once a start exists, clicking this button again is a restart, not a
		// mistaken-first-click correction - relabel to say so. The action
		// itself is unchanged (clearStartConfirmed); only what it reads as.
		row.clearBtn.SetText(common.RestartRaceButtonText)
	} else {
		row.startTime.SetText(common.NoStartTimeText)
		row.clearBtn.SetText(common.ClearButtonText)
	}
	row.restarts.SetText(restartsCell(rec))

	// Status always reflects the canonical team state (race-state-machine.md),
	// not just whether the finish timer has locked this row - the ST's own
	// Start click already moves the race to StateStartRecorded ("On the
	// Water"), before the FT ever opens its clock. res stays the zero value
	// until a finish.json entry exists, which DeriveTeamState already treats
	// as "nothing from the finish side yet" and falls through to rec's state.
	var res store.RaceResult
	if r.finishLog != nil {
		res = r.finishLog.Races[n]
	}
	row.winTime.SetText(winningTimeCell(res))
	row.progress.SetText(raceProgressStatus(rec, res, r.session.Team))

	// Locked once the finish timer has begun this race (persona-plan.md
	// section 9): no changes to the start time while a result is in progress.
	if r.raceLockedByFinish(n) {
		setEnabled(row.startBtn, false)
		setEnabled(row.clearBtn, false)
		row.restoreBtn.Hide()
		return
	}

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

// raceProgressStatus maps a team's timing state to the shared display text
// every race tree uses.
func raceProgressStatus(start store.StartRecord, res store.RaceResult, team persona.Team) string {
	return store.DeriveTeamState(start, res).DisplayText(team)
}

// raceLockedByFinish reports whether the finish timer has begun race n (a
// RaceResult exists in the mirrored finish.json) - guard for the ST
// mutators. The ST is locked out of the row for every state that implies,
// so the disabled buttons - not the status wording - carry the "locked"
// meaning.
func (r *Regatta) raceLockedByFinish(n int) bool {
	if r.finishLog == nil {
		return false
	}
	_, ok := r.finishLog.Races[n]
	return ok
}

func (r *Regatta) refreshFinishRow(row *raceRow) {
	res, timed := r.finishLog.Races[row.raceNumber]

	// A saved or approved result with no recorded start time will never get one;
	// say so rather than leaving the transient "awaiting start" placeholder.
	committed := timed && (res.WinningTime != common.EmptyString || res.Approved)

	rec := r.startLog.Races[row.raceNumber]
	switch {
	case rec.StartedAt != nil:
		row.startTime.SetText(rec.Display)
	case committed:
		row.startTime.SetText(common.StartNotCollectedText)
	default:
		row.startTime.SetText(common.WaitingForStartText)
	}
	row.restarts.SetText(restartsCell(rec))
	row.winTime.SetText(winningTimeCell(res))

	// Status always reflects the canonical team state (race-state-machine.md),
	// not just whether this FT has opened its own clock - a peer ST recording
	// a start already moves the race to StateStartRecorded ("On the Water").
	row.progress.SetText(raceProgressStatus(rec, res, r.session.Team))
}

// restartsCell renders a StartRecord's restart count, or the shared
// placeholder when nothing has happened for this race yet.
func restartsCell(rec store.StartRecord) string {
	if rec.StartedAt == nil && len(rec.Cleared) == 0 {
		return common.NoStartTimeText
	}
	return strconv.Itoa(len(rec.Cleared))
}

// winningTimeCell renders a RaceResult's winning time, or the shared
// placeholder when none has been committed yet.
func winningTimeCell(res store.RaceResult) string {
	if res.WinningTime == common.EmptyString {
		return common.NoStartTimeText
	}
	return res.WinningTime
}

func (r *Regatta) refreshAllRows() {
	for n := range r.rows {
		r.refreshRow(n)
	}
	r.refreshStaleLaneLegend()
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
