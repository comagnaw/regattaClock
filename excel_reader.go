package regattaClock

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// RaceEntry represents a single entry in a race
type RaceEntry struct {
	SchoolName     string
	AdditionalInfo string
	Place          string
	Split          string
	Time           string
}

// RaceData represents the data for a single race
type RaceData struct {
	RaceNumber int
	Lanes      map[int]RaceEntry // Lane number (1-6) to RaceEntry
}

// RegattaData represents the structure of the regatta data we'll read from Excel
type RegattaData struct {
	RegattaName string
	Date        string
	Races       []RaceData
}

// ReadExcelFile reads an Excel file and returns the regatta data
func ReadExcelFile(filePath string) (*RegattaData, error) {
	// Open the Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %v", err)
	}
	defer f.Close()

	// Get the first sheet name
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	// Get merged cells
	mergedCells, err := f.GetMergeCells(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get merged cells: %v", err)
	}

	// Create RegattaData
	data := &RegattaData{
		Races: make([]RaceData, 0),
	}

	// Find the title merged cell (A1:I2)
	for _, mc := range mergedCells {
		if mc.GetStartAxis() == "A1" && mc.GetEndAxis() == "I2" {
			// Found our title cell
			value := mc.GetCellValue()
			// Split the value into title and date
			parts := strings.Split(value, "                                                                                                                                                                                                                                                                   ")
			if len(parts) >= 2 {
				data.RegattaName = strings.TrimSpace(parts[0])
				data.Date = strings.TrimSpace(parts[1])
			} else {
				data.RegattaName = strings.TrimSpace(value)
			}
			break
		}
	}

	// Find race number cells (5-row merged cells in column A)
	for _, mc := range mergedCells {
		start := mc.GetStartAxis()
		end := mc.GetEndAxis()

		// Check if it's a 5-row merged cell in column A
		if strings.HasPrefix(start, "A") && strings.HasPrefix(end, "A") {
			startRow := getRowNumber(start)
			endRow := getRowNumber(end)

			if endRow-startRow == 4 { // 5 rows (inclusive)
				// Get the race number
				value := mc.GetCellValue()
				raceNum, err := strconv.Atoi(value)
				if err == nil {
					// Create a new race with lanes
					race := RaceData{
						RaceNumber: raceNum,
						Lanes:      make(map[int]RaceEntry),
					}

					// Process each lane (columns D through I)
					for lane := 1; lane <= 6; lane++ {
						col := rune('A' + lane + 2) // D=1, E=2, F=3, G=4, H=5, I=6
						entry := RaceEntry{}

						// Get data for each row in the lane
						for row := startRow; row <= endRow; row++ {
							// Get the cell value
							cellValue, _ := f.GetCellValue(sheetName, fmt.Sprintf("%c%d", col, row))

							// Assign data based on row position
							switch row - startRow {
							case 0: // First row - School Name
								entry.SchoolName = strings.TrimSpace(cellValue)
							case 1: // Second row - Additional Info
								entry.AdditionalInfo = strings.TrimSpace(cellValue)
							case 2: // Third row - Place
								entry.Place = strings.TrimSpace(cellValue)
							case 3: // Fourth row - Split
								entry.Split = strings.TrimSpace(cellValue)
							case 4: // Fifth row - Time
								entry.Time = strings.TrimSpace(cellValue)
							}
						}

						// Only add the lane if it has a school name or additional info
						if entry.SchoolName != "" || entry.AdditionalInfo != "" {
							race.Lanes[lane] = entry
						}
					}

					// Add the race to our data
					data.Races = append(data.Races, race)
				}
			}
		}
	}

	// Sort races by race number
	sort.Slice(data.Races, func(i, j int) bool {
		return data.Races[i].RaceNumber < data.Races[j].RaceNumber
	})

	// Print the races in order
	fmt.Println("\nRaces in sequential order:")
	for _, race := range data.Races {
		fmt.Printf("\nRace %d:\n", race.RaceNumber)
		// Print lanes in order
		for lane := 1; lane <= 6; lane++ {
			if entry, exists := race.Lanes[lane]; exists {
				fmt.Printf("  Lane %d:\n", lane)
				fmt.Printf("    School: %s\n", entry.SchoolName)
				fmt.Printf("    Additional Info: %s\n", entry.AdditionalInfo)
				fmt.Printf("    Place: %s\n", entry.Place)
				fmt.Printf("    Split: %s\n", entry.Split)
				fmt.Printf("    Time: %s\n", entry.Time)
			}
		}
	}

	return data, nil
}

// Helper function to extract row number from cell reference
func getRowNumber(cellRef string) int {
	row := 0
	for _, c := range cellRef {
		if c >= '0' && c <= '9' {
			row = row*10 + int(c-'0')
		}
	}
	return row
}
