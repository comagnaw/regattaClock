package clock

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/comagnaw/regattaClock/internal/common"
)

type lapTimes []lapTime

func (l lapTimes) hasOOF(row int) bool {
	if oof := l[row].oofLaneNum; oof != common.EmptyString {
		return true
	}
	return false
}

func (l lapTimes) getLaneNum(row int) int {
	if l.hasOOF(row) {
		if laneNum, err := strconv.Atoi(l[row].oofLaneNum); err == nil && laneNum >= 1 && laneNum <= 6 {
			return laneNum
		}
	}
	return 0
}

type lapTime struct {
	place          int
	time           string
	calculatedTime string
	oofLaneNum     string
}

type lapRows []lapRow

type lapRow struct {
	oofEntry    *widget.Entry
	placeButton *widget.Button
	splitEntry  *widget.Entry
	timeLabel   *widget.Label
}


func (c *Clock) oofEntryOnSubmittedFunc(row int) func(text string) {
	return func(text string) {
		if c.isNotRunning() && row < len(c.lapRows) && row < len(c.lapTimes) {
			// Move focus to next row's OOF entry if it exists
			if row+1 < len(c.lapRows) && row+1 < len(c.lapTimes) {
				// Clear any existing text in the next entry
				c.lapRows[row+1].oofEntry.SetText(common.EmptyString)
				// Move focus to the next entry
				c.window.Canvas().Focus(c.lapRows[row+1].oofEntry)
			}
		}
	}
}

func (c *Clock) oofEntryOnChangedFunc(row int) func(text string) {
	return func(text string) {
		if c.isNotRunning() && row < len(c.lapTimes) {
			// Update resultsTable if OOF matches a lane number
			if laneNum, err := strconv.Atoi(text); err == nil && laneNum >= 1 && laneNum <= 6 {
				// Check for duplicate OOF values in other rows
				isDuplicate := false
				for j := 0; j < len(c.lapTimes); j++ {
					if j != row && c.lapTimes[j].oofLaneNum == text {
						isDuplicate = true
						break
					}
				}

				if !isDuplicate {
					// Store previous OOF value before updating
					prevOOF := c.lapTimes[row].oofLaneNum
					// Update the lap time's OOF value
					c.lapTimes[row].oofLaneNum = text
					// Update Place, Split, and Time rows in resultsTable
					c.resultsTable[3][laneNum] = c.lapRows[row].placeButton.Text // Update Place
					c.resultsTable[4][laneNum] = c.lapRows[row].splitEntry.Text  // Update Split
					c.resultsTable[5][laneNum] = c.lapTimes[row].calculatedTime  // Update Time with calculated time
					// Clear previous lane if it was different
					if prevOOF != common.EmptyString && prevOOF != text {
						if prevLaneNum, err := strconv.Atoi(prevOOF); err == nil && prevLaneNum >= 1 && prevLaneNum <= 6 {
							c.resultsTable[3][prevLaneNum] = common.EmptyString // Clear Place
							c.resultsTable[4][prevLaneNum] = common.EmptyString // Clear Split
							c.resultsTable[5][prevLaneNum] = common.EmptyString // Clear Time
						}
					}
					c.window.Content().Refresh()
				} else {
					// If duplicate, clear the input
					c.lapRows[row].oofEntry.SetText(common.EmptyString)
					// Clear the previous lane if it exists
					if prevOOF := c.lapTimes[row].oofLaneNum; prevOOF != common.EmptyString {
						if prevLaneNum, err := strconv.Atoi(prevOOF); err == nil && prevLaneNum >= 1 && prevLaneNum <= 6 {
							c.resultsTable[3][prevLaneNum] = common.EmptyString // Clear Place
							c.resultsTable[4][prevLaneNum] = common.EmptyString // Clear Split
							c.resultsTable[5][prevLaneNum] = common.EmptyString // Clear Time
							c.window.Content().Refresh()
						}
					}
					c.lapTimes[row].oofLaneNum = common.EmptyString
				}
			} else {
				// If OOF is cleared or invalid, clear the previous lane
				prevOOF := c.lapTimes[row].oofLaneNum
				// Update the lap time's OOF value
				c.lapTimes[row].oofLaneNum = text
				if prevOOF != common.EmptyString {
					if prevLaneNum, err := strconv.Atoi(prevOOF); err == nil && prevLaneNum >= 1 && prevLaneNum <= 6 {
						c.resultsTable[3][prevLaneNum] = common.EmptyString // Clear Place
						c.resultsTable[4][prevLaneNum] = common.EmptyString // Clear Split
						c.resultsTable[5][prevLaneNum] = common.EmptyString // Clear Time
						c.window.Content().Refresh()
					}
				}
			}
		}
	}
}

func (c *Clock) placeButtonOnTappedFunc(row int) func() {
	return func() {
		if c.isNotRunning() && len(c.lapTimes) > 0 && len(c.lapTimes) > row {
			// Get the lane number from OOF
			oof := c.lapTimes[row].oofLaneNum
			if oof == common.EmptyString {
				return // Don't allow editing if no lane is assigned
			}

			laneNum, err := strconv.Atoi(oof)
			if err != nil || laneNum < 1 || laneNum > 6 {
				return // Invalid lane number
			}

			// Create a dialog to edit the place value
			currentPlace := c.resultsTable[3][laneNum]
			options := []string{"DNS", "DNF", "DQ", "Next Place"}

			selectWidget := widget.NewSelect(options, func(value string) {
				// Store the old place value
				oldPlace := c.resultsTable[3][laneNum]

				// Handle DQ/DNF/DNS status
				if value == "DQ" || value == "DNF" || value == "DNS" {
					// Update the place value in the results table
					c.resultsTable[3][laneNum] = value

					// Clear Split and Time values in results table
					c.resultsTable[4][laneNum] = common.EmptyString
					c.resultsTable[5][laneNum] = common.EmptyString

					// If the new place is DQ, adjust other place values
					if value == "DQ" {
						// Convert old place to number if possible
						if oldPlaceNum, err := strconv.Atoi(oldPlace); err == nil {
							// Decrease place values greater than the DQ'd place
							for l := 1; l <= 6; l++ {
								if l != laneNum {
									if placeStr := c.resultsTable[3][l]; placeStr != common.EmptyString {
										if placeNum, err := strconv.Atoi(placeStr); err == nil && placeNum > oldPlaceNum {
											c.resultsTable[3][l] = fmt.Sprintf("%d", placeNum-1)
											// Update the corresponding place button
											for i := range c.lapTimes {
												if c.lapTimes[i].oofLaneNum == fmt.Sprintf("%d", l) {
													c.lapRows[i].placeButton.SetText(c.resultsTable[3][l])
													break
												}
											}
										}
									}
								}
							}
						}
					} else if value == "Next Place" {
						// First, update the current lane to Next Place
						c.resultsTable[3][laneNum] = "Next Place"

						// Restore Split and Time values from the lap table
						for i := range c.lapTimes {
							if c.lapTimes[i].oofLaneNum == fmt.Sprintf("%d", laneNum) {
								c.resultsTable[4][laneNum] = c.lapRows[i].splitEntry.Text
								c.resultsTable[5][laneNum] = c.lapTimes[i].calculatedTime
								break
							}
						}

						// Now rescan and reassign all place numbers based on lap times sequence
						nextPlace := 1
						for i := range c.lapTimes {
							if oof := c.lapTimes[i].oofLaneNum; oof != common.EmptyString {
								if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
									placeStr := c.resultsTable[3][laneNum]
									if placeStr != "DQ" && placeStr != "DNS" && placeStr != "DNF" && placeStr != common.EmptyString {
										c.resultsTable[3][laneNum] = fmt.Sprintf("%d", nextPlace)
										// Update the corresponding place button
										c.lapRows[i].placeButton.SetText(c.resultsTable[3][laneNum])
										nextPlace++
									}
								}
							}
						}
					}
				} else if value == "Next Place" {
					// First, update the current lane to Next Place
					c.resultsTable[3][laneNum] = "Next Place"

					// Restore Split and Time values from the lap table
					for i := range c.lapTimes {
						if c.lapTimes[i].oofLaneNum == fmt.Sprintf("%d", laneNum) {
							c.resultsTable[4][laneNum] = c.lapRows[i].splitEntry.Text
							c.resultsTable[5][laneNum] = c.lapTimes[i].calculatedTime
							break
						}
					}

					// Now rescan and reassign all place numbers based on lap times sequence
					nextPlace := 1
					for i := range c.lapTimes {
						if oof := c.lapTimes[i].oofLaneNum; oof != common.EmptyString {
							if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
								placeStr := c.resultsTable[3][laneNum]
								if placeStr != "DQ" && placeStr != "DNS" && placeStr != "DNF" && placeStr != common.EmptyString {
									c.resultsTable[3][laneNum] = fmt.Sprintf("%d", nextPlace)
									// Update the corresponding place button
									c.lapRows[i].placeButton.SetText(c.resultsTable[3][laneNum])
									nextPlace++
								}
							}
						}
					}
				}

				// Update the place button text
				c.lapRows[row].placeButton.SetText(c.resultsTable[3][laneNum])

				// Refresh the window content to show the updated place values
				c.window.Content().Refresh()
			})

			// Set the current value if it exists in options
			for _, option := range options {
				if option == currentPlace {
					selectWidget.SetSelected(option)
					break
				}
			}

			dialog.ShowCustom(
				"Edit Place",
				"Close",
				selectWidget,
				c.window,
			)
		}
	}
}

func (c *Clock) splitEntryOnChangedFunc(row int, timeAdjustment time.Duration) func(text string) {
	return func(text string) {
		if c.isNotRunning() && row < len(c.lapTimes) {
			// Update the lap time
			c.lapTimes[row].time = text

			// Calculate and update the time label
			lapTime, err := parseTime(text)
			if err == nil {
				if timeAdjustment != 0 {
					adjustedTime := lapTime + timeAdjustment
					adjustedTimeStr := formatTime(adjustedTime)
					c.lapRows[row].timeLabel.SetText(adjustedTimeStr)
					c.lapTimes[row].calculatedTime = adjustedTimeStr
				} else {
					c.lapRows[row].timeLabel.SetText(formatTime(lapTime))
					c.lapTimes[row].calculatedTime = formatTime(lapTime)
				}
			}

			// Update resultsTable if OOF matches a lane number
			if oof := c.lapTimes[row].oofLaneNum; oof != common.EmptyString {
				if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
					// Update Place, Split, and Time rows in resultsTable
					c.resultsTable[3][laneNum] = c.lapRows[row].placeButton.Text // Update Place
					c.resultsTable[4][laneNum] = text                            // Update Split
					c.resultsTable[5][laneNum] = c.lapTimes[row].calculatedTime  // Update Time
					c.window.Content().Refresh()
				}
			}
		}
	}
}
