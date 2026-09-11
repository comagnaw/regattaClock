package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/comagnaw/regattaClock/internal/reader"
)

// runReconcile is Milestone 1: compare an xlsm workbook against a captured
// /bulk JSON file and write a plain-language HTML report. It never reaches
// the network - --bulk-file is a file already captured with
// `rcprobe bulk --out`.
func runReconcile(argv []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	var xlsmPath, bulkFile, reportOut string
	fs.StringVar(&xlsmPath, "xlsm", "", "path to the regatta's .xlsm/.xlsx workbook (required)")
	fs.StringVar(&bulkFile, "bulk-file", "", "path to a captured /bulk JSON file, e.g. from `rcprobe bulk --out` (required)")
	fs.StringVar(&reportOut, "report-out", "", "path to write the HTML report (required; keep it outside the repo, or under a gitignored path - it will name real people)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: rcreconcile reconcile --xlsm PATH --bulk-file PATH --report-out PATH\n\nflags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if xlsmPath == "" || bulkFile == "" || reportOut == "" {
		fs.Usage()
		return fmt.Errorf("reconcile: --xlsm, --bulk-file and --report-out are all required")
	}

	rd, err := reader.ReadExcelFile(xlsmPath)
	if err != nil {
		return fmt.Errorf("read xlsm %q: %w", xlsmPath, err)
	}

	raw, err := os.ReadFile(bulkFile)
	if err != nil {
		return fmt.Errorf("read bulk file %q: %w", bulkFile, err)
	}
	entries, err := bulkEntries(raw)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "rcreconcile: found 0 RegattaCentral entries in", bulkFile+".")
		fmt.Fprintln(os.Stderr, "The /bulk schema is PROVISIONAL (see rcmodel.go). Run:")
		fmt.Fprintln(os.Stderr, "    rcreconcile shape --bulk-file", bulkFile)
		fmt.Fprintln(os.Stderr, "and share the (PII-free) key-path output so bulkEntries can be corrected.")
	}

	matches, unused := matchRaces(rd.SortedRaces(), entries)
	if len(unused) > 0 || countUnmatched(matches) > 0 {
		fmt.Fprintf(os.Stderr, "rcreconcile: %d lane(s) unmatched or ambiguous, %d RegattaCentral entr%s unused - see the report.\n",
			countUnmatched(matches)+countAmbiguous(matches), len(unused), plural(len(unused)))
	}

	return writeReport(reportOut, buildReport(rd.Name, matches, unused))
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

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
