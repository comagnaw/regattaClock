package clock

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/text"
)

// Clock - represents the data used to track the time for a particular race
type Clock struct {

	// clock - the visual digital clock used to time the race
	clock *canvas.Text

	// laps - slice of OOF, Place, Split, Time as fyne widgets
	laps

	// lapCount - used to track progression of race as lap button is pushed
	lapCount int

	// results - bottom table which reflects lane info from raceData and finishing times from laps
	results

	// winningTime - entry field used to collect the final time of the first boat that crosses the finish line
	winningTime *widget.Entry

	// buttons - object which holds various buttons used for clock
	buttons

	// clockState - object used to track progress of clock usage for timing the race
	clockState *clockState

	// RegattaData - data for all races in the scheduled regatta
	RegattaData reader.RegattaData

	// raceData - data that reflects information about one particular race
	raceData reader.RaceData

	// window - reference to the fyne.Window object that makes up the race clock
	window fyne.Window

	// App - reference to the fyne.App object that is running
	App fyne.App
}

// clockState - object used to determine progress of the clock usage for timing the race
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

// NewClock - generates Clock object
func NewClock(parent fyne.App, regattaData *reader.RegattaData, race reader.RaceData) *Clock {

	raceClock := &Clock{
		clock:    text.Header1(common.ZeroTime),
		laps:     make([]lapRow, 6),
		lapCount: 1,
		results:  initResults(race),

		clockState: &clockState{
			isRunning: false,
			isCleared: true,
			stopChan:  make(chan struct{}),
		},
		RegattaData: *regattaData,
		raceData:    race,
		window:      parent.NewWindow(fmt.Sprintf("Race %d Clock", race.RaceNumber)),
		App:         parent,
	}

	raceClock.initButtons()
	raceClock.initWinningTime()

	return raceClock
}

// OpenRaceClock - opens the Clock app so that a race can be timed
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

// startClockUpdate - go routine which displays a running clock once the start button
// is pushed.
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

// getElapsedTime - format of clock as elapsed time from when the clock was started
func (c *Clock) getElapsedTime() string {
	elapsed := time.Since(c.clockState.startTime)
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	tenths := int(elapsed.Milliseconds()/100) % 10
	return fmt.Sprintf(common.ClockFormatter, minutes, seconds, tenths)
}

// isNotRunning - returns true if clockState.isRunning is false
func (c *Clock) isNotRunning() bool {
	return !c.clockState.isRunning
}

// hasWinningTime - return bool based on if winningTime is empty string
func (c *Clock) hasWinningTime() bool {
	return c.winningTime.Text != common.EmptyString
}

// refreshContent - repaints the laps and results
func (c *Clock) refreshContent() {

	// Calculate time adjustments if winning time is set
	var timeAdjustment time.Duration
	if c.hasWinningTime() {
		timeAdjustment, _ = parseTime(c.winningTime.Text)
	}

	for row := range c.laps {

		if !c.laps.emptySplit(row) {
			c.adjustTime(row, timeAdjustment)
		}

		if c.isNotRunning() {
			c.laps[row].oofLaneNum.Enable()
			c.laps[row].split.OnChanged = c.splitEntryOnChangedFunc(row, timeAdjustment)
		} else {
			c.laps[row].oofLaneNum.Disable()
		}

		if laneNum := c.laps.getLaneNum(row); laneNum != badLaneNum {
			c.results.updateFromLapRows(laneNum, row, c.laps)
		}

	}

}

// splitEntryOnChangeFunc - function used to when split is updated by user. This will update
// the calculatedTime in laps, time and splits in results.
func (c *Clock) splitEntryOnChangedFunc(row int, timeAdjustment time.Duration) func(newSplit string) {
	return func(newSplit string) {
		if c.isNotRunning() {
			if laneNum := c.laps.getLaneNum(row); laneNum != badLaneNum && c.results.isPlace(laneNum) {
				c.adjustTime(row, timeAdjustment)
				c.results.updateSplit(laneNum, newSplit)
				c.window.Content().Refresh()
			}
		}
	}
}

// adjustPlaceForNextPlace - iterate over laps and if lap has valid place, ensure each row's place is adjusted
// to sequential order
func (c *Clock) adjustPlaceForNextPlace() {
	newPlace := 1
	for _, lRow := range c.laps {
		if laneNum := getGoodLaneNum(lRow.oofLaneNum.Text); laneNum != badLaneNum {
			if c.results.isNextPlace(laneNum) || c.results.isPlace(laneNum) {
				c.results.updatePlace(laneNum, strconv.Itoa(newPlace))
				lRow.place.Text = strconv.Itoa(newPlace)
				newPlace++
			}
		}
	}
}

// adjustPlaceForDQ - using old place number from DQ'd lane, decrease the place for other non-DQ'd lanes
func (c *Clock) adjustPlaceForDQ(oldPlaceNum int) {
	for _, lRow := range c.laps {
		if laneNum := getGoodLaneNum(lRow.oofLaneNum.Text); laneNum != badLaneNum {
			if currentPlaceNum := getGoodPlaceNum(c.results.place(laneNum)); currentPlaceNum != badLaneNum {
				if currentPlaceNum > oldPlaceNum {
					newPlaceNum := strconv.Itoa(currentPlaceNum - 1)
					c.results.updatePlace(laneNum, newPlaceNum)
					lRow.place.Text = newPlaceNum
				}
			}
		}
	}
}

// adjustTime - with provided row and timeAdjustment, add split and timeAdjustment and
// update laps calculatedTime and results time fields.
func (c *Clock) adjustTime(row int, timeAdjustment time.Duration) {
	if splitTime, err := parseTime(c.laps.split(row)); err == nil {

		adjustedTime := formatTime(splitTime + timeAdjustment)

		c.laps.setCalculatedTime(row, adjustedTime)

		if laneNum := c.laps.getLaneNum(row); laneNum != badLaneNum {
			c.results.updateTime(laneNum, adjustedTime)
		}
	}
}

// parseTime - parses a time string in format "00:00.0" and converts to time.Duration
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

	return 0, fmt.Errorf("invalid time format")
}

// formatTime - formats a time.Duration to "00:00.0"
func formatTime(d time.Duration) string {
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	tenths := int(d.Milliseconds()/100) % 10
	return fmt.Sprintf(common.ClockFormatter, minutes, seconds, tenths)
}
