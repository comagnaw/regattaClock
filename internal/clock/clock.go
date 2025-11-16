package clock

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// App represents the main application
type Clock struct {

	// raceTitle - top title which
	raceTitle *canvas.Text

	// clock the visual digital clock used to time the race
	clock *canvas.Text

	// lapRows slice of OOF, Place, Split, Time is captured as input
	lapRows

	laps int

	// resultsTable - bottom table which reflects lane info from raceData and finishing times from lapTimes
	resultsTable

	// winningTime - entry field used to collect the final time of the first boat that crosses the finish line
	winningTime *widget.Entry

	buttons

	// clockState object used to track progress of clock usage for timing the race
	clockState *clockState

	// RegattaData data for all races in the scheduled regatta
	RegattaData reader.RegattaData

	// raceData data that reflects information about one particular race
	raceData reader.RaceData

	// window reference to the fyne.Window object that makes up the race clock
	window fyne.Window

	// App reference to the fyne.App object that is running
	App fyne.App
}

// clockState object used to determine progress of the clock usage for timing the race
type clockState struct {

	// isRunning the start button has been pushed, the clock time is progressing and the stop button has not been pushed
	isRunning bool

	// isCleared the clear button has been pushed and the clock should return to common.ZeroTime
	isCleared bool

	// startTime the timestamp collected when the start button is selected and used as basis for collecting lap times
	startTime time.Time

	// stopChan go channel used to signal stoppage of the clock
	stopChan chan struct{}
}

// NewClock generates Clock object
func NewClock(parent fyne.App, regattaData *reader.RegattaData, race reader.RaceData) *Clock {

	raceClock := &Clock{
		window:  parent.NewWindow(fmt.Sprintf("Race %d Clock", race.RaceNumber)),
		App:     parent,
		laps:    1,
		lapRows: make([]lapRow, 6),
		clockState: &clockState{
			isRunning: false,
			isCleared: true,
			stopChan:  make(chan struct{}),
		},
		raceData:    race,
		RegattaData: *regattaData,
	}

	raceClock.resultsTable = initResultsTable((raceClock.raceData))
	raceClock.setRaceTitle()
	raceClock.setClock()
	raceClock.initWinningTime()

	return raceClock
}

// OpenRaceClock opens the Clock app so that a race can be timed
func (c *Clock) OpenRaceClock() {

	c.window.SetContent(c.content())
	c.window.Resize(fyne.NewSize(clockWidth, clockHeight))

	// Set up keyboard handler for this window
	c.window.Canvas().SetOnTypedKey(c.setupKeyboardHandler())

	// Start the clock update goroutine for this window
	go c.startClockUpdate()

	// Set up window close handler to clean up the goroutine
	c.window.SetOnClosed(func() {
		close(c.clockState.stopChan)
	})

	c.window.Show()
}

func (c *Clock) setRaceTitle() {
	c.raceTitle = canvas.NewText(c.raceData.RaceTitle(), color.White)
	c.raceTitle.TextStyle = fyne.TextStyle{Bold: true}
	c.raceTitle.Alignment = fyne.TextAlignCenter
	c.raceTitle.TextSize = 48
}

func (c *Clock) setClock() {
	c.clock = canvas.NewText(common.ZeroTime, color.White)
	c.clock.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	c.clock.Alignment = fyne.TextAlignCenter
	c.clock.TextSize = 48
}

func (c *Clock) startClockUpdate() {
	ticker := time.NewTicker(100 * time.Millisecond) // Update every 0.1 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.clockState.isRunning {
				formatted := c.getElapsedTime()
				// Use fyne.Do to update UI on the main thread
				fyne.Do(func() {
					if c.clock != nil { // Add nil check for safety
						c.clock.Text = formatted
						c.clock.Refresh()
					}
				})
			}
		case <-c.clockState.stopChan:
			return
		}
	}
}

func (c *Clock) getElapsedTime() string {
	elapsed := time.Since(c.clockState.startTime)
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	tenths := int(elapsed.Milliseconds()/100) % 10
	return fmt.Sprintf(common.ClockFormatter, minutes, seconds, tenths)
}

func (c *Clock) isNotRunning() bool {
	return !c.clockState.isRunning
}

func (c *Clock) hasWinningTime() bool {
	return c.winningTime.Text != common.EmptyString
}

func (c *Clock) refreshContent() {

	// Calculate time adjustments if winning time is set
	var timeAdjustment time.Duration
	if c.hasWinningTime() {
		timeAdjustment, _ = parseTime(c.winningTime.Text)
	}

	for row := range c.lapRows {

		if !c.lapRows.emptySplit(row) {
			c.adjustTime(row, timeAdjustment)
		}

		if c.isNotRunning() {
			c.lapRows[row].oofLaneNum.Enable()
			c.lapRows[row].split.OnChanged = c.splitEntryOnChangedFunc(row, timeAdjustment)
		} else {
			c.lapRows[row].oofLaneNum.Disable()
		}

		if laneNum := c.lapRows.getLaneNum(row); laneNum != badLaneNum {
			c.resultsTable.updateFromLapRows(laneNum, row, c.lapRows)
		}

	}

}

func (c *Clock) splitEntryOnChangedFunc(row int, timeAdjustment time.Duration) func(newSplit string) {
	return func(newSplit string) {
		if c.isNotRunning() {
			if laneNum := c.lapRows.getLaneNum(row); laneNum != badLaneNum && c.resultsTable.isPlace(laneNum) {
				c.adjustTime(row, timeAdjustment)
				c.resultsTable.updateSplit(laneNum, newSplit)
				c.window.Content().Refresh()
			}
		}
	}
}

func (c *Clock) adjustTime(row int, timeAdjustment time.Duration) {
	if lapTime, err := parseTime(c.lapRows.split(row)); err == nil {

		adjustedTime := formatTime(lapTime + timeAdjustment)

		c.lapRows.setCalculatedTime(row, adjustedTime)

		if laneNum := c.lapRows.getLaneNum(row); laneNum != badLaneNum {
			c.resultsTable.updateTime(laneNum, adjustedTime)
		}
	}
}

// parseTime parses a time string in format "00:00.0" or "00:00:00.000" to time.Duration
func parseTime(timeStr string) (time.Duration, error) {
	if timeStr == common.EmptyString {
		return 0, nil
	}

	// Try parsing as "00:00.0" format first
	parts := strings.Split(timeStr, ":")
	if len(parts) == 2 {
		minutes, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes")
		}

		secondsParts := strings.Split(parts[1], ".")
		if len(secondsParts) != 2 {
			return 0, fmt.Errorf("invalid seconds format")
		}

		seconds, err := strconv.Atoi(secondsParts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds")
		}

		tenths, err := strconv.Atoi(secondsParts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid tenths")
		}

		return time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second +
			time.Duration(tenths)*100*time.Millisecond, nil
	}

	// Try parsing as "00:00:00.000" format
	if len(parts) == 3 {
		hours, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid hours")
		}

		minutes, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes")
		}

		secondsParts := strings.Split(parts[2], ".")
		if len(secondsParts) != 2 {
			return 0, fmt.Errorf("invalid seconds format")
		}

		seconds, err := strconv.Atoi(secondsParts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds")
		}

		milliseconds, err := strconv.Atoi(secondsParts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid milliseconds")
		}

		return time.Duration(hours)*time.Hour +
			time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second +
			time.Duration(milliseconds)*time.Millisecond, nil
	}

	return 0, fmt.Errorf("invalid time format")
}

// formatTime formats a time.Duration to "00:00.0"
func formatTime(d time.Duration) string {
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	tenths := int(d.Milliseconds()/100) % 10
	return fmt.Sprintf(common.ClockFormatter, minutes, seconds, tenths)
}
