package clock

const (
	// clockWidth / clockHeight - the race-clock window. Sized to the results
	// panel (the widest zone) with the operator controls stacked above it. Much
	// smaller than the pre-polish 1240x800: the inputs no longer stretch the
	// whole frame and the results columns are capped with ellipsis truncation.
	clockWidth  = float32(880)
	clockHeight = float32(928)

	// The lanes table has seven columns (a narrow row-label column + six lanes).
	// resultsPanel() derives the lane-column width and the exact viewport size
	// from these plus the live theme padding, so the six lanes fill the Results
	// card edge to edge (no wasted card on the right) and every cell shows with
	// no scrollbars. Long school names ellipsize inside their column rather than
	// widening it (the full name still shows in the Referee Approval window).
	resultsCardInset     = float32(20)                   // total left+right breathing room inside the card
	resultsLabelColWidth = float32(72)                   // the "Place" / "Split" / "Time" row-label column
	resultsRowHeight     = float32(30)                   // each of the six data rows
	resultsWidth         = clockWidth - resultsCardInset // note-line width; not pixel-critical

	// lap grid column widths - shared by the header row and the six data rows so
	// their edges line up. OOF is a single lane digit; Place holds "Next Place" /
	// "DQ" / a number; Split and Time hold mm:ss.s.
	lapOOFColWidth   = float32(64)
	lapPlaceColWidth = float32(184)
	lapSplitColWidth = float32(120)
	lapTimeColWidth  = float32(120)

	// lapRowGap - extra vertical space between lap rows (on top of the layout's
	// own padding) so the input fields are not crowded together.
	lapRowGap = float32(8)

	// zoneBandVPad - breathing room inside the "Timing" / "Results" accent bands
	// (matches the race tree's headerBandVPad).
	zoneBandVPad = float32(4)

	// bandLogoHeight - the regattaClock wordmark that opens the centred Timing
	// band phrase. bandLogoAspect is its SVG viewBox ratio (2793 / 430) so the
	// width follows; bandLogoGap is the space between the wordmark and the
	// "timing for ..." text.
	bandLogoHeight = float32(26)
	bandLogoAspect = float32(2793) / float32(430)
	bandLogoGap    = float32(12)

	// controlGap - horizontal space inserted between the Start / Lap / Stop /
	// Clear buttons so the rapid-tap targets are not crowded together.
	controlGap = float32(24)

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
