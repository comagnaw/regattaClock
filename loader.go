package regattaClock

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

func (a *App) loadExcel(fromStartup bool) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		if reader == nil {
			// User cancelled, show reminder
			if fromStartup {
				dialog.ShowInformation(
					"Load Later",
					"You can load the Excel file later by selecting 'Import Regatta Table' from the menu.",
					a.window,
				)
			}
			return
		}
		defer reader.Close()

		// Verify file extension
		uri := reader.URI()
		if uri.Extension() != ".xlsx" {
			dialog.ShowError(fmt.Errorf("only .xlsx files are supported"), a.window)
			return
		}

		// Get the file path from the URI
		filePath := uri.Path()
		fmt.Printf("Debug: Importing Excel file: %s\n", filePath)

		// Read the Excel file
		regattaData, err := ReadExcelFile(filePath)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		// Store the regatta data
		a.regattaData = regattaData

		// Calculate scheduled races (races with at least one lane)
		scheduledRaces := 0
		for _, race := range regattaData.Races {
			if len(race.Lanes) > 0 {
				scheduledRaces++
			}
		}

		fmt.Printf("Debug: Successfully loaded regatta data - %d total races, %d scheduled races\n",
			len(regattaData.Races), scheduledRaces)
		fmt.Printf("Debug: Regatta Name: %s\n", regattaData.RegattaName)
		fmt.Printf("Debug: Regatta Date: %s\n", regattaData.Date)

		// Update the title, scheduled races count, and date
		a.regattaTitle.Text = regattaData.RegattaName
		a.scheduledRaces.Text = fmt.Sprintf("Scheduled Races: %d", scheduledRaces)
		a.regattaDate.Text = regattaData.Date
		a.regattaTitle.Refresh()
		a.scheduledRaces.Refresh()
		a.regattaDate.Refresh()

		// Show success message
		dialog.ShowInformation("Import", "Successfully read Excel file", a.window)
	}, a.window)
}
