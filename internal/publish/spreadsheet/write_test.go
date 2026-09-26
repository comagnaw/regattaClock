package spreadsheet

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
)

// exampleWorkbook is the real manual workbook whose Results sheet this
// package's layout reproduces.
const exampleWorkbook = "../../../examples/Example Heat Sheets and Results With Macros.xlsm"

var publishedAt = time.Date(2026, 10, 3, 9, 30, 0, 0, time.UTC)

func fixtureRaces() []publish.PublishableRace {
	return []publish.PublishableRace{
		{
			RaceNumber:    1,
			ScheduledTime: "09:00 AM",
			BoatClass:     "M-1x",
			FlightInfo:    "Heat 1",
			Lanes: map[int]store.ScheduleEntry{
				1: {SchoolName: "Snoopy", AdditionalInfo: "A"},
				2: {SchoolName: "Linus"},
				6: {SchoolName: "Pig Pen", AdditionalInfo: "SCRATCHED", Status: store.StatusScratched},
			},
			Rows: []publish.Row{
				{Lane: "1", Place: "2", Split: "03:01.0", Time: "06:05.0"},
				{Lane: "2", Place: "1", Split: "03:00.0", Time: "06:00.0"},
				{Lane: "", Place: "DNS"}, // no lane - no cell to go in
			},
		},
		{RaceNumber: 2, ScheduledTime: "09:05 AM", BoatClass: "W-2x", Lanes: map[int]store.ScheduleEntry{3: {SchoolName: "Lucy"}}},
	}
}

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName("Charlie Brown Classic", "Saturday, May 30, 2027"))
	ledger := Ledger{1: {Revision: "abc123", PublishedAt: publishedAt}}
	meta := Meta{Name: "Charlie Brown Classic", Date: "Saturday, May 30, 2027", RegattaKey: "key-2027"}
	if err := Write(path, meta, fixtureRaces(), ledger); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	return path
}

func openT(t *testing.T, path string) *excelize.File {
	t.Helper()
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v", path, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestWrite_Layout(t *testing.T) {
	f := openT(t, writeFixture(t))
	sheet := common.ResultsSheetName

	if got := f.GetSheetList(); !slices.Equal(got, []string{sheet, LedgerSheetName}) {
		t.Fatalf("sheets = %v", got)
	}
	if f.GetActiveSheetIndex() != 0 {
		t.Errorf("active sheet = %d, want the Results sheet", f.GetActiveSheetIndex())
	}

	want := map[string]string{
		"A1":  "Charlie Brown Classic Regatta Results",
		"A2":  "Saturday, May 30, 2027",
		"A4":  "#",
		"D4":  "Lane 1",
		"I4":  "Lane 6",
		"A5":  "1",
		"B5":  "09:00 AM",
		"C5":  "M-1x",
		"C6":  "Heat 1",
		"C7":  "Place",
		"C8":  "Split",
		"C9":  "Time",
		"D5":  "Snoopy",
		"D6":  "A",
		"D7":  "2",
		"D8":  "03:01.0",
		"D9":  "06:05.0",
		"E7":  "1",
		"I5":  "Pig Pen",
		"I6":  "SCRATCHED",
		"I7":  "", // scratched, never timed
		"A10": "2",
		"C10": "W-2x",
		"F10": "Lucy",
		"C12": "Place",
		"F12": "", // race 2 has no results
	}
	for c, w := range want {
		if got, _ := f.GetCellValue(sheet, c); got != w {
			t.Errorf("%s = %q, want %q", c, got, w)
		}
	}

	// A place is written as a number, so Excel does not flag it as text.
	if typ, _ := f.GetCellType(sheet, "E7"); typ != excelize.CellTypeNumber && typ != excelize.CellTypeUnset {
		t.Errorf("E7 type = %v, want a number", typ)
	}

	merges, _ := f.GetMergeCells(sheet)
	var refs []string
	for _, m := range merges {
		refs = append(refs, m.GetStartAxis()+":"+m.GetEndAxis())
	}
	for _, w := range []string{"A1:I1", "A2:I2", "A5:A9", "B5:B9", "A10:A14", "B10:B14"} {
		if !slices.Contains(refs, w) {
			t.Errorf("merges %v missing %s", refs, w)
		}
	}
}

func TestWrite_LedgerIsVeryHidden(t *testing.T) {
	f := openT(t, writeFixture(t))
	if visible, _ := f.GetSheetVisible(LedgerSheetName); visible {
		t.Error("ledger sheet is visible, want very hidden")
	}
}

func TestRead_RoundTrip(t *testing.T) {
	got, err := Read(writeFixture(t))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if !got.Exists || got.RegattaKey != "key-2027" {
		t.Errorf("Exists/RegattaKey = %v/%q, want true/key-2027", got.Exists, got.RegattaKey)
	}
	if e := got.Ledger[1]; e.Revision != "abc123" || !e.PublishedAt.Equal(publishedAt) {
		t.Errorf("Ledger[1] = %+v", e)
	}
	if len(got.Ledger) != 1 {
		t.Errorf("Ledger = %+v, want only race 1", got.Ledger)
	}

	rows := got.Rows[1]
	if len(rows) != 2 {
		t.Fatalf("Rows[1] = %+v, want lanes 1 and 2", rows)
	}
	wantLane2 := publish.Row{Lane: "2", School: "Linus", Place: "1", Split: "03:00.0", Time: "06:00.0"}
	if rows[1] != wantLane2 {
		t.Errorf("Rows[1][1] = %+v, want %+v", rows[1], wantLane2)
	}
	if _, ok := got.Rows[2]; ok {
		t.Error("Rows[2] present, want only ledger races read back")
	}
}

func TestRead_MissingFileIsEmpty(t *testing.T) {
	got, err := Read(filepath.Join(t.TempDir(), "nope.xlsx"))
	if err != nil || got.Exists || len(got.Ledger) != 0 {
		t.Errorf("Read(missing) = %+v, %v; want empty, nil", got, err)
	}
}

func TestRead_ForeignWorkbookRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hand-kept.xlsx")
	f := excelize.NewFile()
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if _, err := Read(path); !errors.Is(err, ErrForeignWorkbook) {
		t.Errorf("Read(foreign) error = %v, want ErrForeignWorkbook", err)
	}
}

func TestWrite_RegeneratesWhole(t *testing.T) {
	path := writeFixture(t)
	races := fixtureRaces()[:1]
	races[0].Rows = nil
	if err := Write(path, Meta{Name: "X"}, races, Ledger{}); err != nil {
		t.Fatal(err)
	}
	f := openT(t, path)
	if got, _ := f.GetCellValue(common.ResultsSheetName, "A10"); got != "" {
		t.Errorf("A10 = %q after rewriting with one race, want the old block gone", got)
	}
	if got, _ := f.GetCellValue(common.ResultsSheetName, "E7"); got != "" {
		t.Errorf("E7 = %q, want cleared", got)
	}
}

func TestWrite_ReadOnlyFolderIsLocked(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not block writes here")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := Write(filepath.Join(dir, "r.xlsx"), Meta{}, nil, nil)
	if !errors.Is(err, ErrLocked) {
		t.Errorf("Write(read-only dir) error = %v, want ErrLocked", err)
	}
}

func TestFileName_SanitizedAndDated(t *testing.T) {
	if got := FileName(`Fall: "Classic"`, "10/3/2026"); got != "Fall_ _Classic_ - 10_3_2026 Results.xlsx" {
		t.Errorf("FileName() = %q", got)
	}
	if got := FileName("Fall Classic", ""); got != "Fall Classic Results.xlsx" {
		t.Errorf("FileName(no date) = %q", got)
	}
	if FileName("Fall Classic", "2026-10-03") == FileName("Fall Classic", "2027-10-02") {
		t.Error("same-named regattas in different years share a file name")
	}
}

func TestPublished_CheckRegatta(t *testing.T) {
	got, err := Read(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := got.CheckRegatta("key-2027"); err != nil {
		t.Errorf("CheckRegatta(own key) = %v, want nil", err)
	}
	if err := got.CheckRegatta("key-2028"); !errors.Is(err, ErrOtherRegatta) {
		t.Errorf("CheckRegatta(other key) = %v, want ErrOtherRegatta", err)
	}
	if err := (Published{}).CheckRegatta("any"); err != nil {
		t.Errorf("CheckRegatta(no file) = %v, want nil", err)
	}
	if err := (Published{Exists: true}).CheckRegatta("any"); !errors.Is(err, ErrOtherRegatta) {
		t.Errorf("CheckRegatta(unkeyed workbook) = %v, want ErrOtherRegatta", err)
	}
}

// TestLayout_MatchesExampleWorkbook pins this package's geometry to the real
// manual Results worksheet it reproduces: the header row, the first two
// blocks' origins (by race number), and column C's Place/Split/Time labels.
func TestLayout_MatchesExampleWorkbook(t *testing.T) {
	f := openT(t, exampleWorkbook)
	sheet := common.ResultsSheetName

	for i, want := range HeaderLabels() {
		got, _ := f.GetCellValue(sheet, cell(i+1, HeaderRow))
		if strings.TrimSpace(got) != want {
			t.Errorf("header col %d = %q, want %q", i+1, got, want)
		}
	}
	for i, race := range []string{"1", "2"} {
		origin := BlockOrigin(i)
		if got, _ := f.GetCellValue(sheet, cell(RaceNumCol, origin)); got != race {
			t.Errorf("block %d origin A%d = %q, want race %s", i, origin, got, race)
		}
		for off := RowPlace; off <= RowTime; off++ {
			if got, _ := f.GetCellValue(sheet, cell(LabelCol, origin+off)); got != BlockLabels[off] {
				t.Errorf("C%d = %q, want %q", origin+off, got, BlockLabels[off])
			}
		}
	}
}
