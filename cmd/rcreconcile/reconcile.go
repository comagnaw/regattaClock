package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/comagnaw/regattaClock/internal/reader"
)

// runReconcile is Milestone 1: compare an xlsm workbook against a directory of
// captured RegattaCentral JSON files and write a plain-language HTML report.
// It never reaches the network - --rc-dir is a directory already populated by
// `rcprobe walk` (or the individual `rcprobe bulk` / `entries` commands),
// --out pointed at the same directory.
func runReconcile(argv []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	var xlsmPath, rcDir, reportOut, uploadPreviewOut string
	var debugRace int
	var guessTies bool
	fs.StringVar(&xlsmPath, "xlsm", "", "path to the regatta's .xlsm/.xlsx workbook (required)")
	fs.StringVar(&rcDir, "rc-dir", "", "directory of captured RegattaCentral JSON files, e.g. from `rcprobe walk --out` (required)")
	fs.StringVar(&reportOut, "report-out", "", "path to write the HTML report (required; keep it outside the repo, or under a gitignored path - it will name real people)")
	fs.StringVar(&uploadPreviewOut, "upload-preview-out", "", "optional: path to write a local dry-run preview of the RegattaCentral upload this data would produce - never sent, Client.Upload is never called; keep it outside the repo or under a gitignored path")
	fs.IntVar(&debugRace, "debug-race", 0, "optional: print a step-by-step trace (to stderr) of how this one race number was matched - school/org names and entry ids only, never an athlete's name")
	fs.BoolVar(&guessTies, "guess-ties", false, "optional: as a last resort, pick one entry for a lane that stays ambiguous after every other signal is tried (e.g. two boats from the same school in the same class) - off by default; the report still marks these \"best guess\", not a confident match")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: rcreconcile reconcile --xlsm PATH --rc-dir DIR --report-out PATH [--upload-preview-out PATH] [--debug-race N] [--guess-ties]\n\nflags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if xlsmPath == "" || rcDir == "" || reportOut == "" {
		fs.Usage()
		return fmt.Errorf("reconcile: --xlsm, --rc-dir and --report-out are all required")
	}

	rd, err := reader.ReadExcelFile(xlsmPath)
	if err != nil {
		return fmt.Errorf("read xlsm %q: %w", xlsmPath, err)
	}

	entries, events, err := entriesFromDir(rcDir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "rcreconcile: found 0 RegattaCentral entries in", rcDir+".")
		fmt.Fprintln(os.Stderr, "The capture is likely missing per-event entries - try:")
		fmt.Fprintln(os.Stderr, "    rcprobe walk <regattaID> --out", rcDir)
		fmt.Fprintln(os.Stderr, "(a real schema mismatch on bulk.json/organizations.json/events.json would have")
		fmt.Fprintln(os.Stderr, "already failed above with a decode error, not silently reached this message - if")
		fmt.Fprintln(os.Stderr, "that happens instead, run: rcreconcile shape --bulk-file", filepath.Join(rcDir, "bulk.json"))
		fmt.Fprintln(os.Stderr, "and compare its (PII-free) key-path output to internal/regattacentral/readmodel.go.)")
	}

	heatSheet, hsStats, err := readHeatSheet(xlsmPath)
	if err != nil {
		return fmt.Errorf("read heat sheet: %w", err)
	}
	switch {
	case !hsStats.SheetFound:
		fmt.Fprintln(os.Stderr, "rcreconcile: no worksheet named exactly \"Heat Sheet\" found - rower-name disambiguation is unavailable for this run.")
	case hsStats.ThreeRowBlocks == 0:
		fmt.Fprintf(os.Stderr, "rcreconcile: found the Heat Sheet tab (%d race block(s)), but none were the expected 3-row shape - rower-name disambiguation is unavailable. If this workbook's Heat Sheet layout differs from cmd/rcreconcile/heatsheet.go's assumption, share the row-count shape (no real values needed) so it can be adjusted.\n", hsStats.RaceBlocksSeen)
	case hsStats.RowerNamesFound == 0:
		fmt.Fprintf(os.Stderr, "rcreconcile: found the Heat Sheet tab (%d 3-row race block(s)), but no rower name in row 3 of any of them - rower-name disambiguation is unavailable for this run (expected if this regatta has no 1x/2x boats).\n", hsStats.ThreeRowBlocks)
	default:
		fmt.Fprintf(os.Stderr, "rcreconcile: found %d rower name(s) and %d lane-class annotation(s) on the Heat Sheet tab.\n",
			hsStats.RowerNamesFound, hsStats.LaneClassesFound)
	}

	races := rd.SortedRaces()
	if debugRace != 0 {
		found := false
		for _, race := range races {
			if race.RaceNumber == debugRace {
				traceRace(os.Stderr, race, entries, events, heatSheet)
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "rcreconcile: --debug-race %d: no race with that number in the xlsm.\n", debugRace)
		}
	}

	matches, unused := matchRaces(races, entries, events, heatSheet, guessTies)
	if len(unused) > 0 || countUnmatched(matches) > 0 {
		fmt.Fprintf(os.Stderr, "rcreconcile: %d lane(s) unmatched or ambiguous, %d RegattaCentral entr%s unused - see the report.\n",
			countUnmatched(matches)+countAmbiguous(matches), len(unused), plural(len(unused)))
	}
	if n := countGuessed(matches); n > 0 {
		fmt.Fprintf(os.Stderr, "rcreconcile: %d lane(s) resolved with --guess-ties - the report marks these \"best guess\", not a confident match.\n", n)
	}

	if err := writeReport(reportOut, buildReport(rd.Name, matches, unused)); err != nil {
		return err
	}

	if uploadPreviewOut != "" {
		req, warnings := buildUploadPreview(matches)
		if len(warnings) > 0 {
			fmt.Fprintf(os.Stderr, "rcreconcile: %d lane(s) in the upload preview use a placeholder id - see %s.\n",
				len(warnings), uploadPreviewOut)
		}
		if err := writeUploadPreview(uploadPreviewOut, req, warnings); err != nil {
			return err
		}
	}

	return nil
}

func countUnmatched(matches []laneMatch) int {
	n := 0
	for _, m := range matches {
		if m.Status == statusUnmatched {
			n++
		}
	}
	return n
}

func countAmbiguous(matches []laneMatch) int {
	n := 0
	for _, m := range matches {
		if m.Status == statusAmbiguous {
			n++
		}
	}
	return n
}

func countGuessed(matches []laneMatch) int {
	n := 0
	for _, m := range matches {
		if m.Status == statusGuessed {
			n++
		}
	}
	return n
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
