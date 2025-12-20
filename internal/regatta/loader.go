package regatta

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// loader - load excel file using dialog.
func (r *Regatta) loader(fromStartup bool) {
	dialog.ShowFileOpen(r.callback(fromStartup), r.window)
}

// callback - function used as callback on loader.
func (r *Regatta) callback(fromStartup bool) func(fyne.URIReadCloser, error) {
	return func(fileReader fyne.URIReadCloser, err error) {

		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}

		if fileReader == nil {
			// User cancelled, show reminder
			if fromStartup {
				dialog.ShowInformation(
					"Load Later",
					"You can load the Excel file later by selecting 'Import Regatta Table' from the menu.",
					r.window,
				)
			}
			return
		}
		defer fileReader.Close()

		filePath, err := getFilePath(fileReader)
		if err != nil {
			dialog.ShowError(err, r.window)
		}

		err = r.setRegattaData(filePath)
		if err != nil {
			dialog.ShowError(err, r.window)
		}

		r.saveRegattaData()

		r.debugLoader()
		r.refreshContent()

		dialog.ShowInformation("Import", "Successfully read Excel file", r.window)

		r.showRaceTree()
	}
}

func getFilePath(fileReader fyne.URIReadCloser) (string, error) {
	uri := fileReader.URI()
	if uri.Extension() != ".xlsx" {
		return common.EmptyString, fmt.Errorf("only .xlsx files are supported")
	}

	return uri.Path(), nil
}

func (r *Regatta) setRegattaData(filePath string) error {
	regattaData, err := reader.ReadExcelFile(filePath)
	if err != nil {
		return err
	}

	r.RegattaData = regattaData
	return nil
}

// debugLoader - console debug messages for loader method
func (r *Regatta) debugLoader() {
	fmt.Printf("Debug: Successfully loaded regatta data - %d total races, %d scheduled races\n",
		len(r.RegattaData.Races), r.RegattaData.ScheduledRaces())
	fmt.Printf("Debug: Regatta Name: %s\n", r.RegattaData.Name)
	fmt.Printf("Debug: Regatta Date: %s\n", r.RegattaData.Date)
	fmt.Printf("Debug: Regatta Source Info: %v\n", r.RegattaData.SourceInfo)
}
