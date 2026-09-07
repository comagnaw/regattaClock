package clock

const (
	// clockWidth - width of clock container
	clockWidth = float32(1240)

	// clockHeight - height of clock container
	clockHeight = float32(800)

	// resultsHeight - height of results table
	resultsHeight = float32(240)

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
