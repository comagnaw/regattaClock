package regatta

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2/dialog"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// scheduleWriteDecision classifies a candidate schedule (r.RegattaData, freshly
// parsed) against whatever regattaSchedule.json is already on disk, before the
// Regatta Director overwrites it (persona-plan.md 3b, "A different regatta is
// not a schedule change").
type scheduleWriteDecision int

const (
	writeFresh              scheduleWriteDecision = iota // nothing on disk yet
	writeSameRegatta                                     // RegattaKeys match - a normal edit
	replaceDifferentRegatta                              // keys differ, no timing data - confirm, keep the old file aside
	blockDifferentRegatta                                // keys differ and timing data exists - refuse
	blockUnreadable                                      // the on-disk schedule will not parse - cannot compare
)

// classifyScheduleWrite compares r.RegattaData's RegattaKey to the on-disk
// schedule's. It touches only the filesystem (no dialogs), so it is the unit
// under test. The returned Schedule is the on-disk one when it was readable.
func (r *Regatta) classifyScheduleWrite(session persona.Session) (scheduleWriteDecision, *store.Schedule) {
	existing, err := store.LoadSchedule(session)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return writeFresh, nil
	case err != nil:
		return blockUnreadable, nil
	}

	newKey := store.RegattaKey(r.RegattaData.Name, r.RegattaData.Date)
	if existing.Key() == newKey {
		return writeSameRegatta, existing
	}
	if regattaHasTimingData(session.Root) {
		return blockDifferentRegatta, existing
	}
	return replaceDifferentRegatta, existing
}

// guardScheduleWrite runs classifyScheduleWrite and, on a clear path, writes the
// schedule and calls proceed. A blocked path shows an explanatory dialog and
// proceed is never called; the "different regatta, no timing" path confirms
// first and keeps the outgoing file as regattaSchedule.<oldKey>.json.
func (r *Regatta) guardScheduleWrite(proceed func()) {
	session, ok := r.directorSession()
	if !ok {
		return
	}

	decision, existing := r.classifyScheduleWrite(session)
	switch decision {
	case writeFresh, writeSameRegatta:
		r.saveRegattaData()
		proceed()

	case blockUnreadable:
		_, cause := store.LoadSchedule(session)
		dialog.ShowError(fmt.Errorf(common.ExistingScheduleUnreadable, session.SchedulePath(), cause), r.window)

	case blockDifferentRegatta:
		applog.Warn("import blocked: different regatta with timing data present",
			"component", "loader", "on_disk", existing.Key(),
			"workbook", store.RegattaKey(r.RegattaData.Name, r.RegattaData.Date))
		dialog.ShowInformation(common.DifferentRegattaTitle,
			fmt.Sprintf(common.DifferentRegattaBlockedMessage,
				existing.Name, existing.Date, r.RegattaData.Name, r.RegattaData.Date),
			r.window)

	case replaceDifferentRegatta:
		dialog.ShowConfirm(common.DifferentRegattaTitle,
			fmt.Sprintf(common.DifferentRegattaReplaceMessage,
				existing.Name, existing.Date, r.RegattaData.Name, r.RegattaData.Date, existing.Key()),
			func(yes bool) {
				if !yes {
					return
				}
				r.setAsideSchedule(session, existing.Key())
				r.saveRegattaData()
				proceed()
			}, r.window)
	}
}

// regattaHasTimingData reports whether either team has written a start.json or
// finish.json under root - the signal that replacing the schedule in place
// would orphan real data.
func regattaHasTimingData(root string) bool {
	for _, team := range timingTeams {
		s := directorTeamSession(root, team)
		if filesystem.FileExists(s.StartPath()) || filesystem.FileExists(s.FinishPath()) {
			return true
		}
	}
	return false
}

// setAsideSchedule renames the current regattaSchedule.json to
// regattaSchedule.<oldKey>.json so a mistaken replace can be recovered.
func (r *Regatta) setAsideSchedule(session persona.Session, oldKey string) {
	src := session.SchedulePath()
	dst := filepath.Join(filepath.Dir(src), fmt.Sprintf("regattaSchedule.%s.json", oldKey))
	if err := os.Rename(src, dst); err != nil {
		applog.Warn("could not set aside the previous schedule",
			"component", "loader", "file", src, "err", err)
		return
	}
	applog.Info("previous schedule set aside", "component", "loader", "moved_to", dst, "key", oldKey)
}
