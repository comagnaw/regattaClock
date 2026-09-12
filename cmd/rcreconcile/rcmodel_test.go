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

	entries, _, err := entriesFromDir(dir)
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

	entries, _, err := entriesFromDir(dir)
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

// TestEntriesFromDirDoesNotLetBulkJSONShadowRealOrgs is a regression test:
// RC ids very likely restart at 1 per entity kind, so an unrelated
// id-plus-name object in bulk.json (here, standing in for e.g. an event or
// regatta object) can coincidentally share an id with a real organization.
// Only organizations.json (the dedicated listing) may populate the org index,
// so that collision can never shadow the real name.
func TestEntriesFromDirDoesNotLetBulkJSONShadowRealOrgs(t *testing.T) {
	dir := t.TempDir()
	// bulk.json sorts first and has an unrelated object with id "1" that
	// would otherwise look like an org to asOrg.
	writeJSON(t, dir, "bulk.json", `{"regatta":{"id":1,"name":"Not An Organization"},"entries":[{"id":"1","organizationId":"1"}]}`)
	writeJSON(t, dir, "organizations.json", `[{"id":1,"name":"Springfield High School"}]`)

	entries, _, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if len(entries) != 1 || entries[0].OrgName != "Springfield High School" {
		t.Errorf("entries = %+v, want the entry resolved to the real org, not bulk.json's regatta object", entries)
	}
}

func TestIsOrganizationsFileAndIsEventsFile(t *testing.T) {
	if !isOrganizationsFile("organizations.json") || isOrganizationsFile("bulk.json") {
		t.Error("isOrganizationsFile misclassified a filename")
	}
	if !isEventsFile("events.json") {
		t.Error("isEventsFile should match events.json")
	}
	if isEventsFile("entries-4.json") {
		t.Error("isEventsFile must not match an entries-<id>.json file")
	}
}

func TestEntriesFromDirMissingDir(t *testing.T) {
	if _, _, err := entriesFromDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func TestEntriesFromDirEmpty(t *testing.T) {
	entries, events, err := entriesFromDir(t.TempDir())
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %v, want none", entries)
	}
	if len(events) != 0 {
		t.Errorf("events = %v, want none", events)
	}
}

func TestEntriesFromDirTagsEventIDFromFilename(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "entries-10.json", `[{"id":"1","organizationId":42}]`)
	writeJSON(t, dir, "bulk.json", `{"id":"2","organizationId":42}`) // no event-id-shaped filename

	entries, _, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	byID := map[string]rcEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	if got := byID["1"].EventID; got != "10" {
		t.Errorf("entry from entries-10.json: EventID = %q, want 10", got)
	}
	if got := byID["2"].EventID; got != "" {
		t.Errorf("entry from bulk.json: EventID = %q, want blank (no filename hint)", got)
	}
}

func TestEntriesFromDirBuildsEventsIndex(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "events.json", `[
		{"id":10,"name":"Varsity 8"},
		{"id":11,"boatClass":"Junior Varsity 8"}
	]`)

	_, events, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if events["10"] != "Varsity 8" || events["11"] != "Junior Varsity 8" {
		t.Errorf("events = %v, want {10: Varsity 8, 11: Junior Varsity 8}", events)
	}
}

// TestEntriesFromDirBackfillsEventIDAcrossDuplicates is a regression test for
// a real-world bug: bulk.json sorts ahead of entries-<id>.json and nests the
// same entries (asEntry now matches an org-id-only reference, so bulk.json's
// copy is recognized too), but bulk.json's copy never carries a
// filename-derived EventID. Before this fix, "first occurrence wins" meant
// bulk.json's blank EventID always won, silently starving entriesForRace's
// event-scoping (bestMatchingEvent) for every real entry - reproducing
// "every school shows N duplicate copies of itself as ambiguous" even after
// event-scoping was added. EventID must backfill from a later duplicate
// while other fields (here, OrgName) keep the documented first-occurrence-wins
// behavior.
func TestEntriesFromDirBackfillsEventIDAcrossDuplicates(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "bulk.json", `{"entries":[
		{"id":"1","organization":{"name":"Springfield High School"}}
	]}`)
	writeJSON(t, dir, "entries-10.json", `[
		{"id":"1","organization":{"name":"SHOULD NOT WIN - bulk.json sorts first"}}
	]`)

	entries, _, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1: %+v", len(entries), entries)
	}
	if got := entries[0].OrgName; got != "Springfield High School" {
		t.Errorf("OrgName = %q, want the bulk.json version to still win", got)
	}
	if got := entries[0].EventID; got != "10" {
		t.Errorf("EventID = %q, want backfilled from entries-10.json", got)
	}
}

func TestAsEntryExtractsParticipantNames(t *testing.T) {
	m := map[string]any{
		"id":      "61",
		"orgName": "Springfield High School",
		"entryParticipants": []any{
			map[string]any{"participantId": "1", "name": "Alex Mihalovich"},
			map[string]any{"participantId": "2", "name": "Jamie Smith"},
		},
	}
	e, ok := asEntry(m)
	if !ok {
		t.Fatal("asEntry() = false, want a recognized entry")
	}
	want := []string{"Alex Mihalovich", "Jamie Smith"}
	if len(e.ParticipantNames) != len(want) || e.ParticipantNames[0] != want[0] || e.ParticipantNames[1] != want[1] {
		t.Errorf("ParticipantNames = %v, want %v", e.ParticipantNames, want)
	}
}

func TestAsEntryNoParticipantsFieldLeavesParticipantNamesNil(t *testing.T) {
	e, ok := asEntry(map[string]any{"id": "1", "orgName": "Springfield High School"})
	if !ok {
		t.Fatal("asEntry() = false, want a recognized entry")
	}
	if e.ParticipantNames != nil {
		t.Errorf("ParticipantNames = %v, want nil", e.ParticipantNames)
	}
}

func TestEventIDFromFilename(t *testing.T) {
	tests := map[string]string{
		"entries-10.json":    "10",
		"entries-abc.json":   "abc",
		"bulk.json":          "",
		"organizations.json": "",
		"entries-.json":      "",
	}
	for in, want := range tests {
		if got := eventIDFromFilename(in); got != want {
			t.Errorf("eventIDFromFilename(%q) = %q, want %q", in, got, want)
		}
	}
}
