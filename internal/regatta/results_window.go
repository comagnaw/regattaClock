package regatta

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/clock"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// openResultsWindow opens (or focuses) the read-only results window for race
// n, for the Director and Awards roles - the primary team's approved result,
// exactly as it was presented to the Referee (clock.ApprovalGrid renders the
// identical OOF/Place/Split/Time/School grid the Referee Approval window
// shows, not the live-clock-styled view Compare Secondary uses). A no-op if
// the race or an approved result can't be found; the row's own View Results
// button is already disabled in that case (refreshDirectorRow), so this is a
// defensive re-check, not the normal path.
func (r *Regatta) openResultsWindow(n int) {
	if r.resultsWindow != nil {
		r.resultsWindow.RequestFocus()
		return
	}
	race, ok := r.raceByNumber(n)
	if !ok {
		return
	}
	tt := r.teamLogs[persona.TeamPrimary]
	if tt == nil || tt.finish == nil {
		return
	}
	res, ok := tt.finish.Races[n]
	if !ok || !store.CanPublish(res) {
		return
	}

	w := r.App.NewWindow(fmt.Sprintf(common.ResultsWindowTitleFormat, n))
	w.SetContent(r.resultsBody(w, race, res))
	w.Resize(fyne.NewSize(resultsWinWidth, resultsWinHeight))
	w.CenterOnScreen()
	w.SetOnClosed(func() { r.resultsWindow = nil })
	r.resultsWindow = w
	w.Show()
}

// resultsBody wraps clock.ApprovalGrid in clock.ApprovalWindowContent - the
// same light-themed layout the Referee Approval window uses - with a single
// Close button in place of Approve/Cancel, per awards.md's explicit "a
// single Close button and no other controls."
func (r *Regatta) resultsBody(w fyne.Window, race reader.RaceData, res store.RaceResult) fyne.CanvasObject {
	grid := clock.ApprovalGrid(race, res.Rows)
	closeBtn := widget.NewButton(common.CloseButtonText, w.Close)
	footer := container.NewHBox(layout.NewSpacer(), closeBtn, layout.NewSpacer())
	return clock.ApprovalWindowContent(race.RaceTitle(), grid, footer)
}
