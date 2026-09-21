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
	"github.com/comagnaw/regattaClock/internal/reader"
)

// legacyRegattaData builds a schedule-shaped RegattaData - RaceEntry no
// longer has result fields to drop, so this is just an ordinary schedule
// fixture (used to prove a second launch leaves an untouched legacy file
// alone in TestMigrationIsNoOpWhenScheduleExists).
func legacyRegattaData() *reader.RegattaData {
	rd := reader.NewRegattaData()
	rd.Name = "Old Regatta"
	rd.Date = "2025-05-01"
	rd.SourceInfo = reader.SourceInfo{Type: "excel", URI: "old.xlsx", Hash: "deadbeef"}
	rd.Races = []reader.RaceData{
		{
			RaceNumber:    4,
			ScheduledTime: "09:00 AM",
			BoatClass:     "Varsity 8",
			FlightInfo:    "Final",
			BoatCount:     2,
			Lanes: map[int]reader.RaceEntry{
				1: {SchoolName: "Alpha", AdditionalInfo: "A"},
				2: {SchoolName: "Beta"},
			},
		},
	}
	return rd
}

// TestDiffSchedule_ScheduledTimeChangeSetsMeta - a scheduled-time-only edit
// should trip the same "Schedule changed" conflict mark as a BoatClass or
// FlightInfo edit (both are race metadata, not lane/result data).
func TestDiffSchedule_ScheduledTimeChangeSetsMeta(t *testing.T) {
	old := &reader.RegattaData{Races: []reader.RaceData{
		{RaceNumber: 1, ScheduledTime: "09:00 AM"},
	}}
	cur := &reader.RegattaData{Races: []reader.RaceData{
		{RaceNumber: 1, ScheduledTime: "09:15 AM"},
	}}

	changes := diffSchedule(old, cur)
	ch, ok := changes[1]
	if !ok || !ch.meta {
		t.Fatalf("expected race 1 to be flagged with meta = true, got %+v (ok=%v)", ch, ok)
	}
}

// TestDiffSchedule_ScratchViaStatus - a lane transitioning to
// Status: StatusScratched, with SchoolName unchanged, must fire
// ch.scratch - not ch.moved, which is what a bare AdditionalInfo-text
// change used to fall into before Status existed.
func TestDiffSchedule_ScratchViaStatus(t *testing.T) {
	old := &reader.RegattaData{Races: []reader.RaceData{
		{RaceNumber: 1, Lanes: map[int]reader.RaceEntry{
			1: {SchoolName: "Rangers", AdditionalInfo: ""},
		}},
	}}
	cur := &reader.RegattaData{Races: []reader.RaceData{
		{RaceNumber: 1, Lanes: map[int]reader.RaceEntry{
			1: {SchoolName: "Rangers", AdditionalInfo: "SCRATCHED", Status: reader.StatusScratched},
		}},
	}}

	changes := diffSchedule(old, cur)
	ch, ok := changes[1]
	if !ok {
		t.Fatal("expected race 1 to be flagged as changed")
	}
	if !ch.scratch {
		t.Errorf("expected ch.scratch = true for a Status flip, got %+v", ch)
	}
	if ch.moved {
		t.Errorf("a real scratch should not also be classified as moved, got %+v", ch)
	}
	if !ch.lanes[1] {
		t.Errorf("lane 1 should be marked changed, got %+v", ch.lanes)
	}
}

// TestDiffSchedule_PlainNoteIsMoved - an AdditionalInfo change that is NOT a
// scratch (e.g. an A/B flight designator) still classifies as ch.moved, not
// ch.scratch - activeBoat only cares about StatusScratched specifically.
func TestDiffSchedule_PlainNoteIsMoved(t *testing.T) {
	old := &reader.RegattaData{Races: []reader.RaceData{
		{RaceNumber: 1, Lanes: map[int]reader.RaceEntry{
			1: {SchoolName: "Capitals", AdditionalInfo: ""},
		}},
	}}
	cur := &reader.RegattaData{Races: []reader.RaceData{
		{RaceNumber: 1, Lanes: map[int]reader.RaceEntry{
			1: {SchoolName: "Capitals", AdditionalInfo: "B"},
		}},
	}}

	changes := diffSchedule(old, cur)
	ch, ok := changes[1]
	if !ok {
		t.Fatal("expected race 1 to be flagged as changed")
	}
	if ch.scratch {
		t.Errorf("a plain note change must not be classified as a scratch, got %+v", ch)
	}
	if !ch.moved {
		t.Errorf("expected ch.moved = true, got %+v", ch)
	}
}

// legacyDataJSON is a frozen pre-persona data.json shape, hand-written
// rather than built from the live RaceData/RaceEntry structs - those no
// longer have Place/Split/Time/Approved/Saved to construct it with, and a
// migration test should exercise a real historical on-disk shape rather
// than whatever the current struct happens to allow. Unmarshaling this into
// today's reader.RegattaData silently drops the unknown result fields,
// which is exactly the migration behavior this test verifies.
const legacyDataJSON = `{
	"Name": "Old Regatta",
	"Date": "2025-05-01",
	"Type": "excel",
	"URI": "old.xlsx",
	"Hash": "deadbeef",
	"Races": [
		{
			"RaceNumber": 4,
			"ScheduledTime": "09:00 AM",
			"BoatClass": "Varsity 8",
			"FlightInfo": "Final",
			"BoatCount": 2,
			"Approved": true,
			"Saved": true,
			"RawData": [["Varsity 8","","","","","",""],["Final","","","","","",""],["","","","","","",""]],
			"Lanes": {
				"1": {"SchoolName": "Alpha", "AdditionalInfo": "A", "Place": "1", "Split": "00:00.0", "Time": "06:00.0"},
				"2": {"SchoolName": "Beta", "AdditionalInfo": "", "Place": "2", "Split": "00:03.0", "Time": "06:03.0"}
			}
		}
	]
}`

func TestMigratesLegacyDataFile(t *testing.T) {
	app := test.NewTempApp(t)
	regattaDir := t.TempDir()
	app.Preferences().SetString(common.PrefRegattaDir, regattaDir)

	dataRoot := filepath.Join(regattaDir, common.RegattaDataDir)
	legacyPath := filepath.Join(dataRoot, common.LegacyDataFile)
	if err := filesystem.CreateDirs(dataRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(legacyDataJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewDirector(app)

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
	NewDirector(app)
	// Recreate the legacy file; a second launch must not touch it because the
	// schedule now exists.
	if err := filesystem.SaveJSONFile(legacyRegattaData(), legacyPath); err != nil {
		t.Fatal(err)
	}
	NewDirector(app)

	if !filesystem.FileExists(legacyPath) {
		t.Error("second launch renamed a legacy file it should have ignored")
	}
}
