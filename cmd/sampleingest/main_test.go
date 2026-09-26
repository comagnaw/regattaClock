package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/comagnaw/regattaClock/internal/sample"
)

const example = "../../examples/Example Heat Sheets and Results With Macros.xlsm"

func TestRun_WritesFixture(t *testing.T) {
	out := filepath.Join(t.TempDir(), "fixture.json")
	var stdout bytes.Buffer
	if err := run([]string{"-in", example, "-out", out, "-seed", "7"}, &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := sample.Parse(b)
	if err != nil {
		t.Fatalf("written fixture does not parse: %v", err)
	}
	if fx.Name != "Sample Regatta Day" {
		t.Errorf("name = %q", fx.Name)
	}
	if !strings.Contains(stdout.String(), "fixture written") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRun_RequiresIn(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("run without -in succeeded")
	}
}

func TestRun_MissingWorkbook(t *testing.T) {
	out := filepath.Join(t.TempDir(), "fixture.json")
	if err := run([]string{"-in", "nope.xlsm", "-out", out}, &bytes.Buffer{}); err == nil {
		t.Fatal("run with a missing workbook succeeded")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a fixture was written for a failed ingest")
	}
}
