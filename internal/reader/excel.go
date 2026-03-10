package reader

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/xuri/excelize/v2"
)

// ReadExcelFile reads an Excel file and returns the regatta data
func ReadExcelFile(filePath string) (*RegattaData, error) {
	// Open the Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %v", err)
	}
	defer f.Close()

	excel, err := initExcel(f)
	if err != nil {
		return nil, err
	}

	load(excel)

	debugReadExcelFile(*excel.RegattaData)

	return excel.RegattaData, nil
}

// excel - implments methods that satisfy sourceData interface
type excel struct {
	file        *excelize.File
	sheetName   string
	mergedCells []excelize.MergeCell
	*RegattaData
}

// initExcel - initilize read of excel data
func initExcel(file *excelize.File) (excel, error) {

	var err error
	e := excel{}
	e.file = file

	// Get the first sheet name
	e.sheetName = file.GetSheetName(0)
	if e.sheetName == common.EmptyString {
		return e, fmt.Errorf("no sheets found in Excel file")
	}

	// Get merged cells
	e.mergedCells, err = file.GetMergeCells(e.sheetName)
	if err != nil {
		return e, fmt.Errorf("failed to get merged cells: %v", err)
	}

	e.RegattaData = NewRegattaData()

	return e, nil
}

func (e excel) setNameAndDate() {
	for _, mc := range e.mergedCells {
		if mc.GetStartAxis() == "A1" && mc.GetEndAxis() == "I1" {
			// Found our title
			e.Name = strings.TrimSpace(mc.GetCellValue())
			continue
		}
		if mc.GetStartAxis() == "A2" && mc.GetEndAxis() == "I2" {
			// Found our date
			e.Date = strings.TrimSpace(mc.GetCellValue())
			continue
		}
		if e.Name != common.EmptyString && e.Date != common.EmptyString {
			break
		}
	}
}

func (e excel) loadRaces() {
	for _, mc := range e.mergedCells {
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

					race := e.loadLanes(raceNum, startRow, endRow)

					// Add the race to RegattaData Races
					e.RegattaData.Races = append(e.RegattaData.Races, race)
				}
			}
		}
	}

	// Sort races by race number
	sort.Slice(e.RegattaData.Races, func(i, j int) bool {
		return e.RegattaData.Races[i].RaceNumber < e.RegattaData.Races[j].RaceNumber
	})

}

// loadLanes - parse 5 rows of data and columsn C-I and return RaceData
func (e excel) loadLanes(raceNum, startRow, endRow int) RaceData {

	raceData := newRaceData(raceNum)

	// Get data for each row in the lane
	for row := startRow; row <= endRow; row++ {
		rawDataCols := []string{"C", "D", "E", "F", "G", "H", "I"}
		for i, rawCol := range rawDataCols {
			rawCellValue, _ := e.file.GetCellValue(e.sheetName, fmt.Sprintf("%s%d", rawCol, row))
			raceData.RawData[row-startRow][i] = strings.TrimSpace(rawCellValue)
		}
	}

	raceData.BoatClass = raceData.RawData.getBoatClass()
	raceData.FlightInfo = raceData.RawData.getFlightInfo()

	for lane := 1; lane <= 6; lane++ {
		entry := raceData.RawData.getRaceEntryByLane(lane)

		// Only add entry if it is not considered empty
		if !entry.isEmptyEntry() {
			raceData.Lanes[lane] = entry
			raceData.BoatCount++
		}
	}

	return raceData

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

func debugReadExcelFile(data RegattaData) {
	// Print the races in order
	fmt.Println("\nRaces in sequential order:")
	for _, race := range data.Races {
		fmt.Printf("\nRace %d:\n", race.RaceNumber)
		fmt.Printf("\nBoatClass %s:\n", race.BoatClass)
		fmt.Printf("\nFlight %s:\n", race.FlightInfo)
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

		// Print RawData
		fmt.Println("\n  Raw Data (Columns C through I):")
		for row := 0; row < len(race.RawData); row++ {
			fmt.Printf("    Row %d: ", row+1)
			for col := 0; col < len(race.RawData[row]); col++ {
				fmt.Printf("[%s] ", race.RawData[row][col])
			}
			fmt.Println()
		}
	}
}
