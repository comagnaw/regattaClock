package regatta

import (
	"testing"
	"time"
)

func TestParseRegattaDate(t *testing.T) {
	want := time.Date(2025, 3, 13, 0, 0, 0, 0, time.UTC)

	parsed := []string{
		"2025-03-13",
		"March 13, 2025",
		"Mar 13, 2025",
		want.Format("Monday, January 2, 2006"), // weekday must match the date
		"13 March 2025",
		"3/13/2025",
		"03/13/2025",
		"2025/3/13",
	}
	for _, s := range parsed {
		got, ok := parseRegattaDate(s)
		if !ok {
			t.Errorf("parseRegattaDate(%q): ok=false, want a parse", s)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("parseRegattaDate(%q) = %s, want %s", s, got.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}

	for _, s := range []string{"", "   ", "next weekend", "45729", "2025-13-40", "13/3/2025 morning"} {
		if got, ok := parseRegattaDate(s); ok {
			t.Errorf("parseRegattaDate(%q) = %s, ok=true; want ok=false", s, got)
		}
	}
}

func TestPastRegattaDate(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.Local)

	cases := []struct {
		date     string
		wantPast bool
	}{
		{"2026-06-14", true},       // yesterday
		{"2026-06-15", false},      // today
		{"2026-06-16", false},      // tomorrow
		{"June 1, 2026", true},     // earlier this month, long form
		{"2025-06-15", true},       // a year ago
		{"January 1, 2099", false}, // far future
		{"", false},                // empty - never past
		{"not a date", false},      // unparseable - never past
	}
	for _, c := range cases {
		regatta, past := pastRegattaDate(c.date, now)
		if past != c.wantPast {
			t.Errorf("pastRegattaDate(%q, %s) past=%v, want %v", c.date, now.Format("2006-01-02"), past, c.wantPast)
		}
		if past && regatta.IsZero() {
			t.Errorf("pastRegattaDate(%q) past=true but returned a zero date", c.date)
		}
	}

	// An unparseable/empty date returns a zero time so callers don't format it.
	for _, s := range []string{"", "not a date"} {
		if regatta, _ := pastRegattaDate(s, now); !regatta.IsZero() {
			t.Errorf("pastRegattaDate(%q) = %s, want zero", s, regatta)
		}
	}
}
