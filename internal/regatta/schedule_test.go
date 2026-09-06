package regatta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func legacyRegattaData() *reader.RegattaData {
	rd := reader.NewRegattaData()
	rd.Name = "Old Regatta"
	rd.Date = "2025-05-01"
	rd.SourceInfo = reader.SourceInfo{Type: "excel", URI: "old.xlsx", Hash: "deadbeef"}
	rd.Races = []reader.RaceData{
		{
			RaceNumber: 4,
			BoatClass:  "Varsity 8",
			FlightInfo: "Final",
			BoatCount:  2,
			Approved:   true,
			Saved:      true,
			RawData:    reader.RawData{{"Varsity 8"}, {"Final"}, {"", "1"}, {"", "00:00.0"}, {"", "06:00.0"}},
			Lanes: map[int]reader.RaceEntry{
				1: {SchoolName: "Alpha", AdditionalInfo: "A", Place: "1", Split: "00:00.0", Time: "06:00.0"},
				2: {SchoolName: "Beta", Place: "2", Split: "00:03.0", Time: "06:03.0"},
			},
		},
	}
	return rd
}

func TestScheduleConversionDropsResultFields(t *testing.T) {
	sch := scheduleFromRegattaData(legacyRegattaData())

	if sch.Name != "Old Regatta" || sch.Origin.Hash != "deadbeef" {
		t.Fatalf("metadata not carried: %+v", sch)
	}
	if len(sch.Races) != 1 || sch.Races[0].BoatCount != 2 {
		t.Fatalf("race not carried: %+v", sch.Races)
	}
	if sch.Races[0].Lanes[1] != (store.ScheduleEntry{SchoolName: "Alpha", AdditionalInfo: "A"}) {
		t.Fatalf("lane 1 = %+v, want school/additional only", sch.Races[0].Lanes[1])
	}

	// Back to RegattaData: result fields come back zero.
	rd := regattaDataFromSchedule(sch)
	if rd.Races[0].Approved || rd.Races[0].Saved || rd.Races[0].RawData != nil {
		t.Fatalf("result fields survived the round trip: %+v", rd.Races[0])
	}
	if rd.Races[0].Lanes[1].Place != "" || rd.Races[0].Lanes[2].Time != "" {
		t.Fatalf("lane result fields survived: %+v", rd.Races[0].Lanes)
	}
	if rd.Races[0].Lanes[2].SchoolName != "Beta" {
		t.Fatalf("schedule fields lost: %+v", rd.Races[0].Lanes[2])
	}
}

func TestMigratesLegacyDataFile(t *testing.T) {
	app := test.NewTempApp(t)
	regattaDir := t.TempDir()
	app.Preferences().SetString(common.PrefRegattaDir, regattaDir)

	dataRoot := filepath.Join(regattaDir, common.RegattaDataDir)
	legacyPath := filepath.Join(dataRoot, common.LegacyDataFile)
	if err := filesystem.CreateDirs(dataRoot); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.SaveJSONFile(legacyRegattaData(), legacyPath); err != nil {
		t.Fatal(err)
	}

	r := NewRegatta(app)

	schedulePath := persona.Session{
		Definition: persona.DirectorDefinition,
		Root:       dataRoot,
	}.SchedulePath()

	if !filesystem.FileExists(schedulePath) {
		t.Fatalf("migration did not create %s", schedulePath)
	}
	if filesystem.FileExists(legacyPath) {
		t.Error("legacy data.json should have been renamed aside")
	}
	if !filesystem.FileExists(legacyPath + ".migrated") {
		t.Error("expected data.json.migrated alongside")
	}
	if len(r.RegattaData.Races) != 1 || r.RegattaData.Races[0].RaceNumber != 4 {
		t.Fatalf("migrated schedule not loaded into the session: %+v", r.RegattaData.Races)
	}
	if onWelcome(r) {
		t.Error("a migrated regatta should not land on the welcome view")
	}

	body, err := os.ReadFile(schedulePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"\"Place\"", "\"Approved\"", "\"RawData\"", "\"Saved\""} {
		if strings.Contains(string(body), banned) {
			t.Errorf("regattaSchedule.json still contains %s:\n%s", banned, body)
		}
	}
}

func TestMigrationIsNoOpWhenScheduleExists(t *testing.T) {
	app := test.NewTempApp(t)
	regattaDir := t.TempDir()
	app.Preferences().SetString(common.PrefRegattaDir, regattaDir)

	dataRoot := filepath.Join(regattaDir, common.RegattaDataDir)
	legacyPath := filepath.Join(dataRoot, common.LegacyDataFile)
	if err := filesystem.CreateDirs(dataRoot); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.SaveJSONFile(legacyRegattaData(), legacyPath); err != nil {
		t.Fatal(err)
	}

	// First launch migrates.
	NewRegatta(app)
	// Recreate the legacy file; a second launch must not touch it because the
	// schedule now exists.
	if err := filesystem.SaveJSONFile(legacyRegattaData(), legacyPath); err != nil {
		t.Fatal(err)
	}
	NewRegatta(app)

	if !filesystem.FileExists(legacyPath) {
		t.Error("second launch renamed a legacy file it should have ignored")
	}
}
