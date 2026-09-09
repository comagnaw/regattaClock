package exporter

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/comagnaw/regattaClock/internal/reader"
)

func TestExport_CreatesFiles(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Test Regatta",
		Date: "2024-01-01",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  2,
				BoatClass:  "Varsity 8+",
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
					2: {SchoolName: "School B"},
				},
			},
		},
	}

	Export(regattaData, tmpDir)

	expectedFile := filepath.Join(tmpDir, "race_01_Test_Regatta.png")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedFile)
	}
}

func TestExport_SkipsRacesWithNoBoats(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Test Regatta",
		Date: "2024-01-01",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  0,
				Lanes:      map[int]reader.RaceEntry{},
			},
			{
				RaceNumber: 2,
				BoatCount:  1,
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
				},
			},
		},
	}

	Export(regattaData, tmpDir)

	race1File := filepath.Join(tmpDir, "race_01_Test_Regatta.png")
	if _, err := os.Stat(race1File); !os.IsNotExist(err) {
		t.Errorf("Race with no boats should not create file %s", race1File)
	}

	race2File := filepath.Join(tmpDir, "race_02_Test_Regatta.png")
	if _, err := os.Stat(race2File); os.IsNotExist(err) {
		t.Errorf("Race with boats should create file %s", race2File)
	}
}

func TestExport_PadsRaceNumber(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Test",
		Races: []reader.RaceData{
			{
				RaceNumber: 5,
				BoatCount:  1,
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
				},
			},
		},
	}

	Export(regattaData, tmpDir)

	expectedFile := filepath.Join(tmpDir, "race_05_Test.png")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected zero-padded filename %s was not created", expectedFile)
	}
}

func TestExport_ReplacesSpacesInName(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Polar Bear",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  1,
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
				},
			},
		},
	}

	Export(regattaData, tmpDir)

	expectedFile := filepath.Join(tmpDir, "race_01_Polar_Bear.png")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected filename with underscores %s was not created", expectedFile)
	}
}

func TestExport_SanitizesReservedCharactersInName(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Regatta: Heat?",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  1,
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
				},
			},
		},
	}

	result := Export(regattaData, tmpDir)
	if result.HasErrors() {
		t.Fatalf("Export reported errors: %v", result.Errors)
	}

	expectedFile := filepath.Join(tmpDir, "race_01_Regatta__Heat_.png")
	file, err := os.Open(expectedFile)
	if err != nil {
		t.Fatalf("Expected sanitized filename %s could not be opened: %v", expectedFile, err)
	}
	defer file.Close()

	if _, err := png.Decode(file); err != nil {
		t.Errorf("Exported file %s is not a valid PNG: %v", expectedFile, err)
	}
}

func TestExport_CreatesValidPNG(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Test",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  2,
				BoatClass:  "Varsity 8+",
				FlightInfo: "Heat 1",
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
					3: {SchoolName: "School B"},
				},
			},
		},
	}

	Export(regattaData, tmpDir)

	fileName := filepath.Join(tmpDir, "race_01_Test.png")
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("Failed to open exported file: %v", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("Exported file is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		t.Errorf("Expected image dimensions %dx%d, got %dx%d", width, height, bounds.Dx(), bounds.Dy())
	}
}

func TestExport_LanesOrderedByNumber(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Test",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  3,
				Lanes: map[int]reader.RaceEntry{
					5: {SchoolName: "School E"},
					1: {SchoolName: "School A"},
					3: {SchoolName: "School C"},
				},
			},
		},
	}

	Export(regattaData, tmpDir)

	fileName := filepath.Join(tmpDir, "race_01_Test.png")
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", fileName)
	}
}

func TestExport_MultipleRaces(t *testing.T) {
	tmpDir := t.TempDir()

	regattaData := reader.RegattaData{
		Name: "Test",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  1,
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
				},
			},
			{
				RaceNumber: 10,
				BoatCount:  1,
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School B"},
				},
			},
			{
				RaceNumber: 2,
				BoatCount:  1,
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School C"},
				},
			},
		},
	}

	Export(regattaData, tmpDir)

	expectedFiles := []string{
		filepath.Join(tmpDir, "race_01_Test.png"),
		filepath.Join(tmpDir, "race_02_Test.png"),
		filepath.Join(tmpDir, "race_10_Test.png"),
	}

	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", f)
		}
	}
}
