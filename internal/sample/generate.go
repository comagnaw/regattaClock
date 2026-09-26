package sample

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
	"github.com/comagnaw/regattaClock/internal/reader"
)

const (
	// DefaultUnraced is how many of the day's last races are left un-raced,
	// so testers can run the end of the day on real machines.
	DefaultUnraced = 5

	// HeatSheetFile is the generated Heat Sheet workbook, directly in Dir.
	HeatSheetFile = "Sample Heat Sheet.xlsx"
	// ResultsDirName is the Results workbook's folder under Dir - its own
	// folder, as the results drive is separate from regattaData on race day.
	ResultsDirName = "results"
	// RegattaDataDirName is the regattaData tree under Dir.
	RegattaDataDirName = "regattaData"
)

// ErrDirNotEmpty is returned when Options.Dir already holds files: one run
// never mixes artifacts from two samples.
var ErrDirNotEmpty = errors.New("sample: directory is not empty")

// Options configures Generate.
type Options struct {
	// Dir is the artifact root. Everything is written under it, and nothing
	// anywhere else. Created if absent; refused unless empty.
	Dir string

	// Date is the regatta date. Zero means Now's date - a regatta dated
	// before today is blocked from loading (internal/regatta/date_guard.go).
	Date time.Time

	// Unraced is how many of the last races with boats get no timing; 0
	// means every race is timed. The CLI defaults it to DefaultUnraced.
	Unraced int

	// Now is the current time; zero means time.Now().
	Now time.Time

	// Fixture is the race day to write; nil means the embedded one.
	Fixture *Fixture
}

// Artifact is one file the generator wrote.
type Artifact struct {
	Path string
	Size int64
}

// Report summarizes a generated sample.
type Report struct {
	Races        int // Heat Sheet races, numbered-but-empty ones included
	Scheduled    int // races with boats
	Raced        int // races with timing and published results
	Unraced      int // races with boats and no timing
	FirstUnraced int // 0 when every race is raced
	NoResults    []int
	Artifacts    []Artifact
}

// String renders the report the CLI prints.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Sample regatta written.\n")
	fmt.Fprintf(&b, "  races: %d total, %d with boats, %d raced, %d un-raced\n", r.Races, r.Scheduled, r.Raced, r.Unraced)
	if r.FirstUnraced > 0 {
		fmt.Fprintf(&b, "  first un-raced race: %d\n", r.FirstUnraced)
	}
	if len(r.NoResults) > 0 {
		fmt.Fprintf(&b, "  races with boats but no fixture results (not timed): %v\n", r.NoResults)
	}
	for _, a := range r.Artifacts {
		fmt.Fprintf(&b, "  %s (%d bytes)\n", a.Path, a.Size)
	}
	return b.String()
}

// Generate writes a sample race day under opts.Dir:
//
//	<Dir>/Sample Heat Sheet.xlsx
//	<Dir>/results/<name> - <date> Results.xlsx
//	<Dir>/regattaData/director/regattaSchedule.json
//	<Dir>/regattaData/timing/{primary,secondary}/{start,finish}.json
//
// The schedule is produced as the RD's import would produce it - by reading
// the generated Heat Sheet back through reader.ReadExcelFile - so the two
// always agree. Timing logs are written straight to their files, bypassing
// the persona journal.
func Generate(opts Options) (Report, error) {
	if opts.Dir == "" {
		return Report{}, errors.New("sample: no directory given")
	}
	if opts.Unraced < 0 {
		return Report{}, fmt.Errorf("sample: un-raced count %d is negative", opts.Unraced)
	}
	fx := opts.Fixture
	if fx == nil {
		var err error
		if fx, err = Embedded(); err != nil {
			return Report{}, err
		}
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	date := opts.Date
	if date.IsZero() {
		date = now
	}
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return Report{}, err
	}
	if err := prepareDir(dir); err != nil {
		return Report{}, err
	}

	// 1. Heat Sheet, then the schedule read back from it.
	heatPath := filepath.Join(dir, HeatSheetFile)
	if err := WriteHeatSheet(heatPath, fx, day.Format(common.RegattaDateDisplayLayout)); err != nil {
		return Report{}, err
	}
	rd, err := reader.ReadExcelFile(heatPath)
	if err != nil {
		return Report{}, fmt.Errorf("sample heat sheet does not read back: %w", err)
	}
	sch := store.ScheduleFromRegattaData(rd)

	root := filepath.Join(dir, RegattaDataDirName)
	director := persona.Session{Definition: persona.DirectorDefinition, Root: root}
	if err := store.SaveSchedule(director, sch); err != nil {
		return Report{}, err
	}

	// 2. Which races are timed.
	rep := Report{Races: len(fx.Races)}
	var withBoats []Race
	for _, r := range fx.Races {
		if r.HasBoats() {
			withBoats = append(withBoats, r)
		}
	}
	rep.Scheduled = len(withBoats)
	cut := max(len(withBoats)-opts.Unraced, 0)
	var raced []Race
	for _, r := range withBoats[:cut] {
		if r.HasResults() {
			raced = append(raced, r)
		} else {
			rep.NoResults = append(rep.NoResults, r.RaceNumber)
		}
	}
	rep.Raced = len(raced)
	rep.Unraced = len(withBoats) - cut
	if cut < len(withBoats) {
		rep.FirstUnraced = withBoats[cut].RaceNumber
	}

	// 3. Timing logs.
	t, err := synthesize(sch, raced, day)
	if err != nil {
		return Report{}, err
	}
	resultsDir := filepath.Join(dir, ResultsDirName)
	t.finish[persona.TeamPrimary].ResultsDir = resultsDir

	var paths []string
	for _, team := range []persona.Team{persona.TeamPrimary, persona.TeamSecondary} {
		for _, w := range []struct {
			role persona.Role
			log  any
		}{
			{persona.RoleStart, t.start[team]},
			{persona.RoleFinish, t.finish[team]},
		} {
			s := persona.Session{Definition: definitionFor(w.role, team), Root: root}
			path := s.WritePath()
			if err := filesystem.CreateDirs(filepath.Dir(path)); err != nil {
				return Report{}, err
			}
			if err := filesystem.SaveJSONFileAtomic(w.log, path); err != nil {
				return Report{}, err
			}
			paths = append(paths, path)
		}
	}

	// 4. The Results workbook, as the PFT's publish would leave it with every
	// raced race published.
	if err := filesystem.CreateDirs(resultsDir); err != nil {
		return Report{}, err
	}
	resultsPath := filepath.Join(resultsDir, spreadsheet.FileName(sch.Name, sch.Date))
	if err := writeResults(resultsPath, sch, t.finish[persona.TeamPrimary]); err != nil {
		return Report{}, err
	}

	for _, p := range append([]string{heatPath, resultsPath, director.SchedulePath()}, paths...) {
		info, err := os.Stat(p)
		if err != nil {
			return Report{}, err
		}
		rep.Artifacts = append(rep.Artifacts, Artifact{Path: p, Size: info.Size()})
	}
	return rep, nil
}

// writeResults writes the Results workbook the way internal/regatta's
// publishResults regenerates it: every scheduled race's block, filled for
// each approved race, and a ledger entry per published race at the Revision
// publish.BuildView gives it - so the PFT opens with those races Published.
func writeResults(path string, sch *store.Schedule, fin *store.FinishLog) error {
	races := publish.ScheduleView(sch)
	approved := make(map[int]publish.PublishableRace)
	for _, pr := range publish.BuildView(sch, fin) {
		approved[pr.RaceNumber] = pr
	}

	ledger := spreadsheet.Ledger{}
	for i, race := range races {
		pr, ok := approved[race.RaceNumber]
		if !ok {
			continue
		}
		races[i] = pr
		ledger[pr.RaceNumber] = spreadsheet.LedgerEntry{
			Revision:    pr.Revision,
			PublishedAt: pr.ApprovedAt.Add(2 * time.Minute),
		}
	}
	meta := spreadsheet.Meta{Name: sch.Name, Date: sch.Date, RegattaKey: sch.Key()}
	return spreadsheet.Write(path, meta, races, ledger)
}

// prepareDir creates dir if absent and refuses one that already holds
// anything.
func prepareDir(dir string) error {
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return filesystem.CreateDirs(dir)
	case err != nil:
		return err
	case len(entries) > 0:
		return fmt.Errorf("%w: %s", ErrDirNotEmpty, dir)
	}
	return nil
}
