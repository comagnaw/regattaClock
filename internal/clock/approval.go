package clock

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
)

func (c *Clock) showRefereeeApproval(raceNumber int) {
	dialog.ShowCustomConfirm(
		fmt.Sprintf("Referee Approval - Race %d", raceNumber),
		"Approve",
		"Cancel",
		c.refereeApprovalContent(raceNumber),
		func(approve bool) {
			if approve {
				for i := range c.RegattaData.Races {
					if c.RegattaData.Races[i].RaceNumber == raceNumber {
						c.RegattaData.Races[i].Approved = true
						c.buttons.save.Enable()
						break
					}
				}
			} else {

			}
		},
		c.window,
	)
}

func (c *Clock) refereeApprovalContent(raceNumber int) *fyne.Container {

	title := canvas.NewText(c.raceData.RaceTitle(), color.White)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter
	title.TextSize = 48

	// Create the table data
	tableData := make([][]string, 0)
	headers := []string{"OOF", "Place", "Split", "Time", "School"}
	tableData = append(tableData, headers)

	// First add numerical places in order
	for i := 1; i <= 6; i++ {
		for lane := 1; lane <= 6; lane++ {
			if c.resultsTable[3][lane] == fmt.Sprintf("%d", i) {
				row := []string{
					fmt.Sprintf("%d", lane),
					c.resultsTable[3][lane],
					c.resultsTable[4][lane],
					c.resultsTable[5][lane],
					c.resultsTable[1][lane],
				}
				tableData = append(tableData, row)
			}
		}
	}

	// Then add DQ/DNS/DNF entries
	for lane := 1; lane <= 6; lane++ {
		place := c.resultsTable[3][lane]
		if place == "DQ" || place == "DNS" || place == "DNF" {
			row := []string{
				fmt.Sprintf("Lane %d", lane),
				place,
				c.resultsTable[4][lane],
				c.resultsTable[5][lane],
				c.resultsTable[1][lane],
			}
			tableData = append(tableData, row)
		}
	}

	// Create the table using a grid layout
	table := container.NewGridWithColumns(5)

	// Add all cells to the grid
	for i, row := range tableData {
		for col, cell := range row {
			text := canvas.NewText(cell, color.Black)
			if i == 0 { // Header row
				text.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				text.TextStyle = fyne.TextStyle{Monospace: true}
			}
			// Left align the school column (index 4), center all others
			if col == 4 { // School column
				text.Alignment = fyne.TextAlignLeading
			} else {
				text.Alignment = fyne.TextAlignCenter
			}
			text.TextSize = 48

			// Create a container with alternating background colors
			var bgColor color.Color
			if col%2 == 0 {
				bgColor = color.White
			} else {
				bgColor = color.RGBA{R: 217, G: 217, B: 217, A: 255} // Light gray
			}

			// Create a rectangle for the background
			rect := canvas.NewRectangle(bgColor)
			rect.Resize(fyne.NewSize(200, 100)) // Set a specific size for the rectangle

			// Create a container with the background and text
			cellContainer := container.NewStack(
				rect,
				container.NewPadded(text),
			)
			table.Add(cellContainer)
		}
	}

	// Create the final content
	return container.NewVBox(
		container.NewCenter(title),
		table,
	)

}
