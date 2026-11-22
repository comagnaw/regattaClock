package store

import (
	"strings"
	"time"
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

// ParseRegattaDate leniently parses a free-form Schedule.Date. ok is false for
// an empty or unrecognised string; callers must then skip any date-based gate
// rather than block the load.
func ParseRegattaDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range regattaDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
