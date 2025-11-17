package clock

import (
	"strconv"

	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)

// placeSelection - presents options for place with current value selected 
func (c Clock) placeSelection(row, laneNum int) *widget.Select {
	options := []string{"DNS", "DNF", "DQ", "Next Place"}
	selection := widget.NewSelect(
		options,
		c.placeSelectionFunc(row, laneNum),
	)

	currentPlace := c.resultsTable.place(laneNum)

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
func (c Clock) placeSelectionFunc(row, laneNum int) func(newPlace string) {
	laneNumAsStr := strconv.Itoa(laneNum)
	return func(newPlace string) {

		// Store the old place value
		oldPlace := c.resultsTable.place(laneNum)

		if newPlace == nextPlaceStr {
			// First, update the current lane to Next Place
			c.resultsTable.updatePlace(laneNum, nextPlaceStr)

			// Restore Split and Time values from the lap table
			if lRow := c.laps.getLapRowByLaneNum(laneNumAsStr); lRow != (lapRow{}) {
				c.resultsTable.updateSplit(laneNum, lRow.split.Text)
				c.resultsTable.updateTime(laneNum, lRow.calculatedTime.Text)
			}

			c.adjustPlaceForNextPlace()


		} else { // Handle DQ/DNF/DNS status

			// Update the place value in the results table
			c.resultsTable.updatePlace(laneNum, newPlace)
			c.laps.setPlace(row, newPlace)

			// Clear Split and Time values in results table
			c.resultsTable.updateSplit(laneNum, common.EmptyString)
			c.resultsTable.updateTime(laneNum, common.EmptyString)

			if oldPlaceNum := getGoodLaneNum(oldPlace); oldPlaceNum != badLaneNum {
				c.adjustPlaceForDQ(oldPlaceNum)
			}

		}

		c.window.Content().Refresh()
	}
}
