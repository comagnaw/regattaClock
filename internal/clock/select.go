package clock

import (
	"fmt"
	"fyne.io/fyne/v2/widget"
	"strconv"

	"github.com/comagnaw/regattaClock/internal/common"
)

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

func (c Clock) placeSelectionFunc(row, laneNum int) func(newPlace string) {
	laneNumAsStr := strconv.Itoa(laneNum)
	return func(newPlace string) {

		// Store the old place value
		oldPlace := c.resultsTable.place(laneNum)

		if newPlace == "Next Place" {
			// First, update the current lane to Next Place
			c.resultsTable.updatePlace(laneNum, "Next Place")

			// Restore Split and Time values from the lap table
			for i := range c.lapRows {
				if c.lapRows.oofLaneNum(i) == laneNumAsStr {
					c.resultsTable.updateSplit(laneNum, c.lapRows[i].split.Text)
					c.resultsTable.updateTime(laneNum, c.lapRows[i].calculatedTime.Text)
					break
				}
			}

			// Now rescan and reassign all place numbers based on lap times sequence
			nextPlace := 1
			for i := range c.lapRows {
				if oof := c.lapRows.oofLaneNum(i); oof != common.EmptyString {
					// if oof := c.lapTimes[i].oofLaneNum; oof != common.EmptyString {
					if laneNum := getGoodLaneNum(oof); laneNum != badLaneNum {
						// if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
						placeStr := c.resultsTable.place(laneNum)
						if placeStr != "DQ" && placeStr != "DNS" && placeStr != "DNF" && placeStr != common.EmptyString {

							c.resultsTable.updatePlace(laneNum, fmt.Sprintf("%d", nextPlace))
							// Update the corresponding place button
							c.lapRows.setPlace(i, c.resultsTable.place(laneNum))
							nextPlace++
						}
					}
				}
			}
		} else { // Handle DQ/DNF/DNS status

			// Update the place value in the results table
			c.resultsTable.updatePlace(laneNum, newPlace)

			// Clear Split and Time values in results table
			c.resultsTable.updateSplit(laneNum, common.EmptyString)
			c.resultsTable.updateTime(laneNum, common.EmptyString)

			// If the new place is DQ, adjust other place values
			// Convert old place to number if possible
			if oldPlaceNum := getGoodLaneNum(oldPlace); oldPlaceNum != badLaneNum {
				// Decrease place values greater than the DQ'd place
				for l := 1; l <= 6; l++ {
					if l != laneNum {
						if placeStr := c.resultsTable.place(l); placeStr != common.EmptyString {
							if placeNum, err := strconv.Atoi(placeStr); err == nil && placeNum > oldPlaceNum {
								c.resultsTable.updatePlace(l, fmt.Sprintf("%d", placeNum-1))
								// Update the corresponding place button
								for i := range c.lapRows {
									if c.lapRows[i].oofLaneNum.Text == fmt.Sprintf("%d", l) {
										c.lapRows.setPlace(i, c.resultsTable.place(l))
										break
									}
								}
							}
						}
					}
				}
			}

		}

		// Update the place button text
		c.lapRows.setPlace(row, c.resultsTable.place(laneNum))

		// Refresh the window content to show the updated place values
		c.window.Content().Refresh()
	}
}
