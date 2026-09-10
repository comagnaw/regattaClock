package clock

const (
	// clockWidth / clockHeight - the race-clock window. Sized to the results
	// panel (the widest zone) with the operator controls stacked above it. Much
	// smaller than the pre-polish 1240x800: the inputs no longer stretch the
	// whole frame and the results columns are capped with ellipsis truncation.
	clockWidth  = float32(880)
	clockHeight = float32(876)

	// resultsWidth / resultsHeight - the lanes table: seven columns (row label +
	// six lanes). Long school names ellipsize inside their column rather than
	// widening it (the full name still shows in the Referee Approval window).
	resultsWidth  = float32(824)
	resultsHeight = float32(206)

	// resultsLabelColWidth / resultsLaneColWidth - fixed table columns so the row
	// labels stay narrow and the six lanes share the rest evenly.
	resultsLabelColWidth = float32(74)
	resultsLaneColWidth  = float32(124)

	// lap grid column widths - shared by the header row and the six data rows so
	// their edges line up. OOF is a single lane digit; Place holds "Next Place" /
	// "DQ" / a number; Split and Time hold mm:ss.s.
	lapOOFColWidth   = float32(64)
	lapPlaceColWidth = float32(184)
	lapSplitColWidth = float32(120)
	lapTimeColWidth  = float32(120)
	lapRowHeight     = float32(34)

	// zoneBandVPad - breathing room inside the "Timing" / "Results" accent bands
	// (matches the race tree's headerBandVPad).
	zoneBandVPad = float32(4)

	// winningEntryWidth - the Winning Time entry is a fixed narrow field beside
	// its label, not a full-width form row.
	winningEntryWidth = float32(150)

	// winningNoteHeight - reserved height for the helper line under the Winning
	// Time field. One line covers the common "auto-filled" note so pressing Start
	// never shifts the Lap button; the rarer multi-line skew/stale notes (shown
	// only once the operator has stopped to check the time) may wrap past it.
	winningNoteHeight = float32(24)

	// badLaneNum - used to indicate the lane number text could not be converted to int
	badLaneNum = -1

	// badPlaceNum - used to indicate the place number text could not be converted to int
	badPlaceNum = -1

	// nextPlace - used by place logic to update laps and results to next sequential place value
	nextPlace = "Next Place"

	// Referee Approval window (referee_window.go). An independent, movable,
	// always-light window whose results grid scales its font with the window so
	// it stays large (referees read it from a distance) yet always fits.
	refereeCols             = 5
	refereeWinWidth         = float32(1100)
	refereeWinHeight        = float32(720)
	refereeFontDesign       = float32(48) // font at/above this window width
	refereeFontMin          = float32(28) // font at the narrowest usable width
	refereeCharWidthDivisor = float32(9)  // school-column width / this ≈ font size; tuned to the widest school names
	refereeRowHeightFactor  = float32(1.5)
	refereeColGutter        = float32(16) // inset each grid cell so columns don't bleed together
	refereeMinGridWidth     = float32(640)
)
