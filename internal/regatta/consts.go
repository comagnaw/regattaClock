package regatta

import "time"

const (
	// directorStaleThreshold - the RD stale banner appears once the freshest of
	// the four timing files is older than this; directorStaleInterval is how
	// often that age is re-checked (it is a wall-clock condition, not an event).
	directorStaleThreshold = 10 * time.Minute
	directorStaleInterval  = time.Minute

	// originPollInterval - how often the RD re-hashes the source workbook to
	// notice the origin was touched (persona-plan.md 3b step 1).
	originPollInterval = 45 * time.Second
)

const (
	// regattaWidth is a touch wider than the pre-persona 800 so a start timer
	// row (title + three buttons + collected time) fits without sideways scroll.
	regattaWidth  = float32(900)
	regattaHeight = float32(600)

	// viewMargin - inset keeping view content off the window edge
	viewMargin = float32(20)

	// raceListMinHeight - floor for the scrolling race list. Deliberately well under
	// regattaHeight: the list shares the window with the title header, so a taller
	// floor would push the content past the window and force it to grow.
	raceListMinHeight = float32(120)

	// Race-tree column widths. The header row and every data row wrap each
	// column in a GridWrap of the matching width, so a bold header label sits
	// directly over its column and the values line up down the list.

	// colInset - horizontal padding fixedCell puts inside every column, so a
	// right-aligned value (e.g. the FT "awaiting start" placeholder) keeps a
	// gap from the column to its left (the Time Race button) rather than
	// bleeding onto it.
	colInset = float32(6)

	// startTimeColWidth - the collected / recorded start-time cell, wide enough
	// for "HH:MM:SS.d" and the FT's "awaiting start" placeholder plus the inset.
	// Shared by ST, FT and RD.
	startTimeColWidth = float32(130)

	// actionsColWidth - the start timer's Start Time / Clear / Restore button
	// group (three equal cells).
	actionsColWidth = float32(300)

	// statusColWidth - the ST lock note, the FT progress indicator, and the RD
	// approval indicator ("timing in progress" is the widest text).
	statusColWidth = float32(150)

	// winTimeColWidth - the RD's winning-time cell (fits the "Winning Time"
	// header plus the inset).
	winTimeColWidth = float32(120)

	// restartsColWidth - the RD's restart count.
	restartsColWidth = float32(80)

	// timeRaceColWidth - the finish timer's Time Race button (fits the button
	// plus the inset).
	timeRaceColWidth = float32(120)

	// welcomeBannerWidth, welcomeBannerHeight - the wordmark as a prominent
	// centered header on the welcome-family views. ~6.5:1 to match the SVG's
	// viewBox so ImageFillContain leaves no letterbox gap.
	welcomeBannerWidth  = float32(600)
	welcomeBannerHeight = float32(92)

	// treeWordmarkWidth, treeWordmarkHeight - the wordmark on its own full-width
	// centered row above the race-tree metadata grid. Same ~6.5:1 ratio, sized
	// down for a working screen.
	treeWordmarkWidth  = float32(380)
	treeWordmarkHeight = float32(58)

	// treeRuleThickness - the rule under the wordmark, deliberately heavier than
	// widget.NewSeparator()'s 1px hairline so the wordmark, the details panel and
	// the race list read as three distinct zones.
	treeRuleThickness = float32(2)

	// headerBandVPad - vertical breathing room inside the filled column-header
	// band. Horizontal padding stays 0 so the header columns line up exactly
	// with the data rows below.
	headerBandVPad = float32(4)

	// versionWinWidth / versionWinHeight - the build-info window. Small: seven
	// short Key: Value rows, a source link, and a Close button.
	versionWinWidth  = float32(420)
	versionWinHeight = float32(320)
)
