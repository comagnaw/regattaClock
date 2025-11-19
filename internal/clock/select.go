package clock

import (
	"strconv"

	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)

// placeSelection - presents options for place with current value selected
func (c Clock) placeSelection(row, lane int) *widget.Select {
	options := []string{common.RaceDidNotStart, common.RaceDidNotFinish, common.RaceDisqualification, nextPlace}
	selection := widget.NewSelect(
		options,
		c.placeSelectionFunc(row, lane),
	)

	currentPlace := c.results.place(lane)

	// Set the current value if it exists in options
	for _, option := range options {
		if option == currentPlace {
			selection.SetSelected(option)
			break
		}
	}
	return selection
}

// placeSelectionFunc - function used for place selection, which updates laps and resultsTable based on what is selected
func (c Clock) placeSelectionFunc(row, lane int) func(newPlace string) {
	laneNumAsStr := strconv.Itoa(lane)
	return func(newPlace string) {

		if newPlace == nextPlace {
			// First, update the current lane to Next Place
			c.results.updatePlace(lane, nextPlace)

			// Restore Split and Time values from the lap table
			if lRow := c.laps.getLapRowByLaneNum(laneNumAsStr); lRow != (lapRow{}) {
				c.results.updateSplit(lane, lRow.split.Text)
				c.results.updateTime(lane, lRow.calculatedTime.Text)
			}

			c.adjustPlaceForNextPlace()

		} else { // Handle DQ/DNF/DNS status

			// Store the old place value
			oldPlace := c.results.place(lane)

			// Update the place value in the results table
			c.results.updatePlace(lane, newPlace)
			c.laps.setPlace(row, newPlace)

			// Clear Split and Time values in results table
			c.results.updateSplit(lane, common.EmptyString)
			c.results.updateTime(lane, common.EmptyString)

			if oldPlaceNum := getGoodLaneNum(oldPlace); oldPlaceNum != badLaneNum {
				c.adjustPlaceForDQ(oldPlaceNum)
			}

		}

		c.window.Content().Refresh()
	}
}
