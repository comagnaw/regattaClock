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

func TestEntriesFromDirResolvesOrgIDAgainstOrganizationsFile(t *testing.T) {
	dir := t.TempDir()
	// A real capture had no inline org name at all - only an id reference.
	writeJSON(t, dir, "entries-4.json", `[
		{"id":"1","organizationId":42},
		{"id":"2","organizationId":99}
	]`)
	writeJSON(t, dir, "organizations.json", `[
		{"id":42,"name":"Springfield High School","abbreviation":"SHS"}
	]`)

	entries, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	byID := map[string]rcEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	if len(byID) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(byID), entries)
	}
	if got := byID["1"]; got.OrgName != "Springfield High School" || got.OrgAbbrev != "SHS" {
		t.Errorf("entry 1 = %+v, want resolved against organizations.json", got)
	}
	if got := byID["2"]; got.OrgName != "" || got.OrgID != "99" {
		t.Errorf("entry 2 = %+v, want OrgID kept but OrgName still blank (id 99 has no match)", got)
	}
}

func TestAsOrgDoesNotMatchAReferenceShapedObject(t *testing.T) {
	// An entry that references its org by id must not itself be picked up as
	// an rcOrg - asOrg requires a bare "name" field, which a reference shape
	// ("organizationId": 42) does not have.
	if _, ok := asOrg(map[string]any{"id": "1", "organizationId": float64(42)}); ok {
		t.Error("a reference-only object must not be recognized as an rcOrg")
	}
	org, ok := asOrg(map[string]any{"id": float64(42), "name": "Springfield High School"})
	if !ok || org.ID != "42" || org.Name != "Springfield High School" {
		t.Errorf("asOrg = %+v, %v; want a recognized org", org, ok)
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
