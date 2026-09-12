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
	fs.StringVar(&xlsmPath, "xlsm", "", "path to the regatta's .xlsm/.xlsx workbook (required)")
	fs.StringVar(&rcDir, "rc-dir", "", "directory of captured RegattaCentral JSON files, e.g. from `rcprobe walk --out` (required)")
	fs.StringVar(&reportOut, "report-out", "", "path to write the HTML report (required; keep it outside the repo, or under a gitignored path - it will name real people)")
	fs.StringVar(&uploadPreviewOut, "upload-preview-out", "", "optional: path to write a local dry-run preview of the RegattaCentral upload this data would produce - never sent, Client.Upload is never called; keep it outside the repo or under a gitignored path")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: rcreconcile reconcile --xlsm PATH --rc-dir DIR --report-out PATH [--upload-preview-out PATH]\n\nflags:\n")
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
		fmt.Fprintln(os.Stderr, "Either the capture is missing per-event entries - try:")
		fmt.Fprintln(os.Stderr, "    rcprobe walk <regattaID> --out", rcDir)
		fmt.Fprintln(os.Stderr, "- or the /bulk schema is PROVISIONAL (see rcmodel.go) and needs correcting. Run:")
		fmt.Fprintln(os.Stderr, "    rcreconcile shape --bulk-file", filepath.Join(rcDir, "bulk.json"))
		fmt.Fprintln(os.Stderr, "and share the (PII-free) key-path output so asEntry/asOrg can be corrected.")
	}

	heatSheet, err := readHeatSheet(xlsmPath)
	if err != nil {
		return fmt.Errorf("read heat sheet: %w", err)
	}

	matches, unused := matchRaces(rd.SortedRaces(), entries, events, heatSheet)
	if len(unused) > 0 || countUnmatched(matches) > 0 {
		fmt.Fprintf(os.Stderr, "rcreconcile: %d lane(s) unmatched or ambiguous, %d RegattaCentral entr%s unused - see the report.\n",
			countUnmatched(matches)+countAmbiguous(matches), len(unused), plural(len(unused)))
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

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
