package regatta

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
)

// applyScheduleConflicts records which changed races need the operator's
// attention (own timing exists, or a clock is open) and repaints the banner and
// row marks. Untimed races have already had their labels refreshed silently
// (persona-plan.md 3c).
func (r *Regatta) applyScheduleConflicts(changed map[int]scheduleChange) {
	if r.scheduleConflicts == nil {
		r.scheduleConflicts = map[int]scheduleChange{}
	}
	added := 0
	for n, ch := range changed {
		if !r.raceNeedsScheduleAttention(n) {
			continue
		}
		r.scheduleConflicts[n] = ch
		added++
	}
	if added > 0 {
		applog.Warn("schedule change touches timed races", "component", "race_tree",
			"races", added, "role", string(r.session.Role))
	}
	r.refreshAllRows()
	r.refreshScheduleBanner()
}

// raceNeedsScheduleAttention decides whether a schedule change to race n is more
// than a silent label refresh for this persona.
func (r *Regatta) raceNeedsScheduleAttention(n int) bool {
	switch r.session.Role {
	case persona.RoleStart:
		rec := r.startLog.Races[n]
		return rec.StartedAt != nil || len(rec.Cleared) > 0
	case persona.RoleFinish:
		if _, open := r.openClocks[n]; open {
			return true
		}
		if r.finishLog != nil {
			if _, ok := r.finishLog.Races[n]; ok {
				return true
			}
		}
		// Elevated awareness: the ST already has a start for this race even if
		// the FT has not opened it yet (persona-plan.md 3c).
		if r.startLog != nil {
			if rec, ok := r.startLog.Races[n]; ok && rec.StartedAt != nil {
				return true
			}
		}
	}
	return false
}

// pushScheduleToOpenClocks refreshes the lane labels in any open race clock for
// a changed race, leaving lap rows, OOF, splits, and the winning time untouched
// (persona-plan.md 3c item 3).
func (r *Regatta) pushScheduleToOpenClocks(changed map[int]scheduleChange) {
	for n, clk := range r.openClocks {
		ch, ok := changed[n]
		if !ok {
			continue
		}
		race, ok := r.raceByNumber(n)
		if !ok {
			continue
		}
		clk.UpdateSchedule(race, ch.changedLanes())
	}
}

// scheduleBannerWidget builds the (initially hidden) race-tree schedule-conflict
// banner. showRaceTree calls this while assembling the header, so it is rebuilt
// with the tree; refreshScheduleBanner drives its text and visibility from
// r.scheduleConflicts.
func (r *Regatta) scheduleBannerWidget() *fyne.Container {
	r.scheduleBannerLabel = widget.NewLabel(common.EmptyString)
	r.scheduleBannerLabel.Wrapping = fyne.TextWrapWord
	dismiss := widget.NewButton(common.CloseButtonText, func() {
		r.scheduleConflicts = map[int]scheduleChange{}
		r.refreshAllRows()
		r.scheduleBanner.Hide()
	})
	// Same caution styling as the RD banners (bannerRoot), returned hidden.
	r.scheduleBanner = bannerRoot(container.NewBorder(nil, nil, nil, dismiss, r.scheduleBannerLabel))
	r.refreshScheduleBanner()
	return r.scheduleBanner
}

// refreshScheduleBanner updates the banner text and shows or hides it from the
// current set of unacknowledged conflicts.
func (r *Regatta) refreshScheduleBanner() {
	if r.scheduleBanner == nil {
		return
	}
	if len(r.scheduleConflicts) == 0 {
		r.scheduleBanner.Hide()
		return
	}
	races := make([]int, 0, len(r.scheduleConflicts))
	for n := range r.scheduleConflicts {
		races = append(races, n)
	}
	sort.Ints(races)

	list := make([]string, len(races))
	for i, n := range races {
		list[i] = strconv.Itoa(n)
	}
	joined := strings.Join(list, ", ")

	format := common.ScheduleConflictStartBanner
	if r.session.Role == persona.RoleFinish {
		format = common.ScheduleConflictFinishBanner
	}
	r.scheduleBannerLabel.SetText(fmt.Sprintf(format, joined))
	r.scheduleBanner.Show()
}
