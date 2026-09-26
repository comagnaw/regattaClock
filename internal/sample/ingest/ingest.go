// Package ingest turns a real, hand-kept race-day workbook (Heat Sheet plus a
// filled-in Results sheet) into the obfuscated sample.Fixture the sample
// regatta generator embeds (docs/features/testing/sample-regatta.md). Run once,
// on a developer machine, by cmd/sampleingest: the source workbook names real
// schools and athletes and is never committed - only the fixture is.
//
// The Heat Sheet is read through reader.ReadExcelFile, the RD's own import.
// The Results sheet is read cell by cell through REP's exported layout
// (internal/publish/spreadsheet), not spreadsheet.Read, which accepts only
// RegattaClock-written workbooks. It stays out of internal/reader, whose
// contract is to read the Heat Sheet only.
package ingest

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/sample"
)

// SampleName replaces the real regatta name. The date is dropped; the
// generator applies one at generate time.
const SampleName = "Sample Regatta Day"

// DefaultSeed is cmd/sampleingest's default -seed.
const DefaultSeed = 1

// Report summarizes an ingest for the developer reviewing its output.
type Report struct {
	Races       int   // Heat Sheet races, numbered-but-empty ones included
	Scheduled   int   // races with boats
	WithResults int   // races with boats and Results-sheet places
	NoResults   []int // races with boats and no results - the generator leaves them un-timed
	BreakAfter  []int
	Orphans     []string // Results entries with no Heat Sheet lane; dropped
	Schools     int      // distinct real school names replaced
	Rowers      int      // distinct real rower names replaced
}

// String renders the report cmd/sampleingest prints.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "races: %d total, %d with boats, %d with results\n", r.Races, r.Scheduled, r.WithResults)
	if len(r.NoResults) > 0 {
		fmt.Fprintf(&b, "races with boats but no results (left un-timed): %v\n", r.NoResults)
	}
	if len(r.BreakAfter) > 0 {
		fmt.Fprintf(&b, "breaks after race: %v\n", r.BreakAfter)
	}
	fmt.Fprintf(&b, "names replaced: %d schools, %d rowers\n", r.Schools, r.Rowers)
	for _, o := range r.Orphans {
		fmt.Fprintf(&b, "dropped: %s\n", o)
	}
	return b.String()
}

// laneResult is one Results-sheet lane's finish.
type laneResult struct{ place, split, time string }

// Ingest reads the workbook at path and returns its obfuscated fixture. It
// fails if any output string still contains a real name.
func Ingest(path string, seed int64) (*sample.Fixture, Report, error) {
	rd, err := reader.ReadExcelFile(path)
	if err != nil {
		return nil, Report{}, fmt.Errorf("heat sheet: %w", err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, Report{}, err
	}
	defer f.Close()

	breaks, err := readBreaks(f)
	if err != nil {
		return nil, Report{}, err
	}
	results, err := readResults(f)
	if err != nil {
		return nil, Report{}, err
	}

	real, rep, err := buildReal(rd, results)
	if err != nil {
		return nil, Report{}, err
	}
	real.BreakAfter = breaks
	rep.BreakAfter = breaks

	fx, err := obfuscate(real, rd.Name, seed, &rep)
	if err != nil {
		return nil, Report{}, err
	}
	return fx, rep, nil
}

// buildReal assembles the un-obfuscated fixture: Heat Sheet races and lanes,
// with each lane's Results-sheet finish joined on race number and lane.
func buildReal(rd *reader.RegattaData, results map[int]map[int]laneResult) (*sample.Fixture, Report, error) {
	fx := &sample.Fixture{Name: rd.Name}
	rep := Report{Races: len(rd.Races)}

	onSheet := map[int]map[int]bool{}
	for _, race := range rd.SortedRaces() {
		sched, err := normalizeTime(race.ScheduledTime)
		if err != nil {
			return nil, Report{}, fmt.Errorf("race %d: %w", race.RaceNumber, err)
		}
		out := sample.Race{
			RaceNumber:    race.RaceNumber,
			ScheduledTime: sched,
			BoatClass:     race.BoatClass,
			FlightInfo:    race.FlightInfo,
			Note:          race.RawData[2][0],
		}
		onSheet[race.RaceNumber] = map[int]bool{}
		if len(race.Lanes) > 0 {
			out.Lanes = make(map[int]sample.Lane, len(race.Lanes))
			rep.Scheduled++
		}
		for lane, entry := range race.Lanes {
			res := results[race.RaceNumber][lane]
			out.Lanes[lane] = sample.Lane{
				School:         entry.SchoolName,
				AdditionalInfo: entry.AdditionalInfo,
				Rowers:         race.RawData[2][lane],
				Status:         store.ScheduleEntryStatus(entry.Status),
				Place:          res.place,
				Split:          res.split,
				Time:           res.time,
			}
			onSheet[race.RaceNumber][lane] = true
		}
		if out.HasBoats() {
			if out.HasResults() {
				rep.WithResults++
			} else {
				rep.NoResults = append(rep.NoResults, race.RaceNumber)
			}
		}
		fx.Races = append(fx.Races, out)
	}

	for n, lanes := range results {
		for lane := range lanes {
			if !onSheet[n][lane] {
				rep.Orphans = append(rep.Orphans, fmt.Sprintf("Results race %d lane %d has a finish but no Heat Sheet entry", n, lane))
			}
		}
	}
	sort.Strings(rep.Orphans)
	return fx, rep, nil
}

// obfuscate replaces every real name in fx and runs the leak check over the
// result.
func obfuscate(real *sample.Fixture, realName string, seed int64, rep *Report) (*sample.Fixture, error) {
	leaks := newLeakSet()
	leaks.add(realName)

	var schools, rowers []string
	seenSchool, seenRower := map[string]bool{}, map[string]bool{}
	addRower := func(n string) {
		leaks.add(n)
		if k := normalize(n); !seenRower[k] {
			seenRower[k] = true
			rowers = append(rowers, n)
		}
	}
	for _, race := range real.Races {
		for _, l := range race.Lanes {
			base, _ := splitSchool(l.School)
			leaks.add(l.School)
			leaks.add(base)
			if k := normalize(base); k != "" && !seenSchool[k] {
				seenSchool[k] = true
				schools = append(schools, base)
			}
			for _, n := range append(names(l.Rowers), names(l.AdditionalInfo)...) {
				addRower(n)
			}
		}
	}
	rep.Schools, rep.Rowers = len(schools), len(rowers)

	o := newObfuscator(seed, leaks)
	if err := o.assign(schools, rowers); err != nil {
		return nil, err
	}

	fx := &sample.Fixture{Name: SampleName, BreakAfter: real.BreakAfter}
	for _, race := range real.Races {
		out := race
		if race.Lanes != nil {
			out.Lanes = make(map[int]sample.Lane, len(race.Lanes))
			for lane, l := range race.Lanes {
				l.School = o.school(l.School)
				l.Rowers = o.cell(l.Rowers)
				l.AdditionalInfo = o.cell(l.AdditionalInfo)
				out.Lanes[lane] = l
			}
		}
		fx.Races = append(fx.Races, out)
	}

	if hits := leakCheck(fx, leaks); len(hits) > 0 {
		return nil, fmt.Errorf("leak check failed - real names remain in the output:\n  %s", strings.Join(hits, "\n  "))
	}
	return fx, nil
}

// leakCheck scans every output string for a real name.
func leakCheck(fx *sample.Fixture, leaks *leakSet) []string {
	var hits []string
	check := func(where, s string) {
		if m := leaks.match(s); m != "" {
			hits = append(hits, fmt.Sprintf("%s %q contains %q", where, s, m))
		}
	}
	check("regatta name", fx.Name)
	for _, r := range fx.Races {
		at := fmt.Sprintf("race %d", r.RaceNumber)
		check(at+" time", r.ScheduledTime)
		check(at+" boat class", r.BoatClass)
		check(at+" flight", r.FlightInfo)
		check(at+" note", r.Note)
		for lane, l := range r.Lanes {
			at := fmt.Sprintf("race %d lane %d", r.RaceNumber, lane)
			for field, v := range map[string]string{
				"school": l.School, "additional info": l.AdditionalInfo, "rowers": l.Rowers,
				"place": l.Place, "split": l.Split, "time": l.Time,
			} {
				check(at+" "+field, v)
			}
		}
	}
	sort.Strings(hits)
	return hits
}

// readBreaks finds the Heat Sheet's break blocks - 3-row column-A merges with
// no race number and sample.BreakMarker in column B - and returns the race
// each one follows.
func readBreaks(f *excelize.File) ([]int, error) {
	sheet, err := sheetNamed(f, common.HeatSheetName)
	if err != nil {
		return nil, err
	}
	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		return nil, err
	}

	raceAt := map[int]int{} // block start row -> race number
	var breakRows []int
	for _, mc := range merges {
		col, top, err := excelize.CellNameToCoordinates(mc.GetStartAxis())
		if err != nil || col != 1 {
			continue
		}
		endCol, bottom, err := excelize.CellNameToCoordinates(mc.GetEndAxis())
		if err != nil || endCol != 1 || bottom-top != 2 {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(mc.GetCellValue())); err == nil {
			raceAt[top] = n
			continue
		}
		b, _ := f.GetCellValue(sheet, fmt.Sprintf("B%d", top))
		if strings.EqualFold(strings.TrimSpace(b), sample.BreakMarker) {
			breakRows = append(breakRows, top)
		}
	}

	var after []int
	for _, row := range breakRows {
		prevRow, prev := 0, 0
		for r, n := range raceAt {
			if r < row && r > prevRow {
				prevRow, prev = r, n
			}
		}
		if prev > 0 {
			after = append(after, prev)
		}
	}
	sort.Ints(after)
	return after, nil
}

// readResults reads every Results-sheet block into race number -> lane ->
// finish, keyed by the block's column-A race number (the Results sheet has no
// break blocks, so block position and race number can disagree with the
// Heat Sheet's). Blocks with no race number or no places are skipped.
func readResults(f *excelize.File) (map[int]map[int]laneResult, error) {
	sheet, err := sheetNamed(f, common.ResultsSheetName)
	if err != nil {
		return nil, err
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	out := map[int]map[int]laneResult{}
	for i := 0; spreadsheet.BlockOrigin(i) <= len(rows); i++ {
		origin := spreadsheet.BlockOrigin(i)
		n, err := strconv.Atoi(strings.TrimSpace(value(f, sheet, spreadsheet.RaceNumCol, origin)))
		if err != nil {
			continue
		}
		for lane := 1; lane <= spreadsheet.MaxLanes; lane++ {
			col := spreadsheet.LaneCol(lane)
			place := strings.TrimSpace(value(f, sheet, col, origin+spreadsheet.RowPlace))
			if place == "" {
				continue
			}
			split, err := finishTime(f, sheet, col, origin+spreadsheet.RowSplit)
			if err != nil {
				return nil, fmt.Errorf("results race %d lane %d split: %w", n, lane, err)
			}
			tm, err := finishTime(f, sheet, col, origin+spreadsheet.RowTime)
			if err != nil {
				return nil, fmt.Errorf("results race %d lane %d time: %w", n, lane, err)
			}
			if out[n] == nil {
				out[n] = map[int]laneResult{}
			}
			out[n][lane] = laneResult{place: place, split: split, time: tm}
		}
	}
	return out, nil
}

// finishTime reads a Split or Time cell. The workbook stores them as Excel
// day fractions formatted mm:ss.0; the raw number is converted here, rounded
// to the tenth, rather than trusting Excel's display rounding. A text cell
// must already be MM:SS.t.
func finishTime(f *excelize.File, sheet string, col, row int) (string, error) {
	name, _ := excelize.CoordinatesToCellName(col, row)
	raw, err := f.GetCellValue(sheet, name, excelize.Options{RawCellValue: true})
	if err != nil {
		return "", err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if days, err := strconv.ParseFloat(raw, 64); err == nil {
		tenths := math.Round(days * 24 * 60 * 60 * 10)
		return sample.FormatClock(time.Duration(tenths) * 100 * time.Millisecond), nil
	}
	if _, err := sample.ParseClock(raw); err != nil {
		return "", err
	}
	return raw, nil
}

func value(f *excelize.File, sheet string, col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	v, _ := f.GetCellValue(sheet, name)
	return v
}

// sheetNamed finds a worksheet by name, ignoring case and padding.
func sheetNamed(f *excelize.File, name string) (string, error) {
	for _, s := range f.GetSheetList() {
		if strings.EqualFold(strings.TrimSpace(s), name) {
			return s, nil
		}
	}
	return "", fmt.Errorf("workbook has no %q sheet", name)
}

// normalizeTime returns a scheduled time in the Heat Sheet's zero-padded
// "03:04 PM" shape the generator parses; "" stays "".
func normalizeTime(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{"03:04 PM", "3:04 PM", "15:04", "3:04PM"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("03:04 PM"), nil
		}
	}
	return "", fmt.Errorf("scheduled time %q is not a clock time", s)
}
