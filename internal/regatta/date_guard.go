package regatta

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2/dialog"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// regattaDateLayouts are the shapes the Excel A2 date cell has been seen as -
// Schedule.Date is copied from it verbatim, so its format follows the workbook
// author's cell formatting (the reader's own fixture renders "March 13, 2025").
// Tried in order; a string matching none is treated as unknown and the
// past-date gate is skipped rather than blocking the load.
var regattaDateLayouts = []string{
	"January 2, 2006", // US long date - what the reader actually produces
	"Jan 2, 2006",
	"Monday, January 2, 2006",
	"2 January 2006",
	"2006-01-02", // ISO - test fixtures and the schedule-data-model doc
	"1/2/2006",   // also accepts 03/13/2025
	"2006/1/2",
}

// parseRegattaDate leniently parses a free-form Schedule.Date. ok is false for
// an empty or unrecognised string; callers must then skip any date-based gate
// rather than block the load.
func parseRegattaDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == common.EmptyString {
		return time.Time{}, false
	}
	for _, layout := range regattaDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// pastRegattaDate reports whether the regatta's civil date is strictly before
// the host's current local date. The bare date carries no timezone, so it is
// read in the host's zone (time.Local) - the "triangulation" via the host
// clock. regatta is the parsed date, for display; an unknown or empty date is
// never past and comes back zero.
func pastRegattaDate(date string, now time.Time) (regatta time.Time, past bool) {
	d, ok := parseRegattaDate(date)
	if !ok {
		return time.Time{}, false
	}
	ry, rm, rd := d.Date()
	ny, nm, nd := now.Date()
	regatta = time.Date(ry, rm, rd, 0, 0, 0, 0, time.Local)
	today := time.Date(ny, nm, nd, 0, 0, 0, 0, time.Local)
	return regatta, regatta.Before(today)
}

// confirmRegattaDate is the last gate before a timer session starts: if the
// schedule's date has already passed, the operator may be about to record times
// into a regatta that has already run - and finish.json / start.json is that
// regatta's permanent record. A past date raises a second confirmation; "No"
// returns to the folder picker, "Yes" proceeds and is logged. A same-day,
// future, empty, or unparseable date starts the session unchanged.
func (r *Regatta) confirmRegattaDate(def persona.Definition, session persona.Session, schedule *store.Schedule) {
	regatta, past := pastRegattaDate(schedule.Date, time.Now())
	if !past {
		r.startSession(session, schedule)
		return
	}

	applog.Warn("regatta date is in the past", "component", "startup",
		"regatta", schedule.Name, "date", schedule.Date)

	msg := fmt.Sprintf(common.PastRegattaMessage,
		schedule.Name,
		regatta.Format(common.RegattaDateDisplayLayout),
		time.Now().Format(common.RegattaDateDisplayLayout))

	dialog.ShowConfirm(common.PastRegattaTitle, msg, func(yes bool) {
		if !yes {
			r.pickPersonaDirectory(def)
			return
		}
		applog.Warn("loading a past regatta on operator confirmation",
			"component", "startup", "regatta", schedule.Name, "date", schedule.Date)
		r.startSession(session, schedule)
	}, r.window)
}
