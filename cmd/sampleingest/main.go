// Command sampleingest turns one real race-day workbook - its Heat Sheet
// plus a filled-in Results sheet - into the obfuscated fixture the sample
// regatta generator embeds (internal/sample/data/regatta-day.json). It is
// run once, on a developer machine; see
// docs/features/testing/sample-regatta.md.
//
// It is NOT shipped: release.yml packages only ./cmd/regattaClock. It imports
// no Fyne, so it builds without CGO.
//
// Usage:
//
//	sampleingest -in <day.xlsm> [-out <fixture.json>] [-seed N]
//
// The source workbook names real schools and athletes: keep it in
// sample-source/ (gitignored) and commit only the fixture, after reviewing it.
// The ingest refuses to write a fixture in which any real name survives.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/sample/ingest"
)

// defaultOut is the embedded fixture's path, relative to the repo root.
const defaultOut = "internal/sample/data/regatta-day.json"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sampleingest:", err)
		os.Exit(1)
	}
}

func run(argv []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("sampleingest", flag.ContinueOnError)
	in := fs.String("in", "", "the real race-day workbook (.xlsm/.xlsx) to read")
	out := fs.String("out", defaultOut, "where to write the obfuscated fixture")
	seed := fs.Int64("seed", ingest.DefaultSeed, "obfuscation seed; the same seed gives the same fixture")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if *in == "" {
		fs.Usage()
		return errors.New("-in is required")
	}

	fx, rep, err := ingest.Ingest(*in, *seed)
	if err != nil {
		return err
	}
	b, err := fx.Marshal()
	if err != nil {
		return err
	}
	if err := filesystem.SaveBytesFileAtomic(b, *out); err != nil {
		return err
	}
	fmt.Fprint(stdout, rep.String())
	fmt.Fprintf(stdout, "fixture written: %s (%d bytes) - review it before committing\n", *out, len(b))
	return nil
}
