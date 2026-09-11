// Command rcreconcile is an investigation tool: it balances what
// RegattaCentral has on file for a regatta against the .xlsm heat-sheet /
// results workbook the RD actually produced, and writes a plain-language HTML
// report for a non-technical reader. It is read-only - it never calls
// RegattaCentral's write API - and its report is a local file only, never
// published anywhere. See ./README.md and
// docs/features/personas/heatsheet-rc-pivot-investigation.md.
//
// Like cmd/rcprobe, it is NOT shipped: release.yml packages only
// ./cmd/regattaClock, so this stays out of releases while `go build ./...`
// still compiles it. It is Fyne-free.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rcreconcile:", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	if len(argv) == 0 {
		usage()
		return fmt.Errorf("no command given")
	}
	cmd, rest := argv[0], argv[1:]
	switch cmd {
	case "shape":
		return runShape(rest)
	case "reconcile":
		return runReconcile(rest)
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: rcreconcile <command> [flags]

commands:
  shape      print the key-path structure of a captured /bulk JSON file -
             types only, never values, safe to share - so the reconcile
             parser can be corrected against the real schema
  reconcile  compare an xlsm workbook against a captured /bulk JSON file and
             write a plain-language HTML report; never calls RegattaCentral's
             write API

Run "rcreconcile <command> -h" for a command's flags.
`)
}
