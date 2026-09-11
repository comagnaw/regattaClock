package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEntriesFromDirMergesAndDedupes(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "bulk.json", `{"events":[{"id":1,"entries":[
		{"id":"1","organization":{"name":"Springfield High School"}},
		{"id":"2","organization":{"name":"Shelbyville Rowing Club"}}
	]}]}`)
	// entries-1.json repeats entry "1" (bulk.json sorts first, so its version
	// wins) and adds a new entry "3" that only exists here.
	writeJSON(t, dir, "entries-1.json", `[
		{"id":"1","organization":{"name":"SHOULD NOT WIN - bulk.json sorts first"}},
		{"id":"3","organization":{"name":"Ogdenville Composite"}}
	]`)
	// A non-JSON file in the same directory must be ignored, not error out.
	writeJSON(t, dir, "notes.txt", "not json at all")

	entries, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}

	byID := map[string]rcEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	if len(byID) != 3 {
		t.Fatalf("got %d distinct entries, want 3: %+v", len(byID), entries)
	}
	if got := byID["1"].OrgName; got != "Springfield High School" {
		t.Errorf("entry 1 org = %q, want the bulk.json version to win", got)
	}
	if got := byID["3"].OrgName; got != "Ogdenville Composite" {
		t.Errorf("entry 3 org = %q, want it merged in from entries-1.json", got)
	}
}

func TestEntriesFromDirMissingDir(t *testing.T) {
	if _, err := entriesFromDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func TestEntriesFromDirEmpty(t *testing.T) {
	entries, err := entriesFromDir(t.TempDir())
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %v, want none", entries)
	}
}
