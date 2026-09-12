package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

func writeJSON(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Fixtures below use the confirmed real envelope/field shapes (see
// readmodel.go and heatsheet-rc-pivot-investigation.md's Milestone 4 entry):
// every response is {success, count, data, links, messages}, with bulk.json
// nesting entries under data.events[], and the dedicated listings
// (entries-<id>.json, organizations.json, events.json) carrying a flat array
// directly under data.

func TestEntriesFromDirMergesAndDedupes(t *testing.T) {
	dir := t.TempDir()
	// bulk.json sorts first, so its version of entry "1" wins; entry "3"
	// only exists in entries-1.json and is merged in.
	writeJSON(t, dir, "bulk.json", `{"success":true,"count":1,"data":{"events":[{"eventId":1,"entries":[
		{"entryId":"1","eventId":1,"organizationId":"5","division":"FromBulk"},
		{"entryId":"2","eventId":1,"organizationId":"6","division":"AlsoFromBulk"}
	]}]}}`)
	writeJSON(t, dir, "entries-1.json", `{"success":true,"count":2,"data":[
		{"entryId":"1","eventId":1,"organizationId":"5","division":"SHOULD NOT WIN - bulk.json sorts first"},
		{"entryId":"3","eventId":1,"organizationId":"7","division":"FromEntriesFile"}
	]}`)
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
	if got := byID["1"].BoatClass; got != "FromBulk" {
		t.Errorf("entry 1 division = %q, want the bulk.json version to win", got)
	}
	if got := byID["1"].EventID; got != "1" {
		t.Errorf("entry 1 EventID = %q, want \"1\" from its own eventId field", got)
	}
	if got := byID["3"].BoatClass; got != "FromEntriesFile" {
		t.Errorf("entry 3 division = %q, want it merged in from entries-1.json", got)
	}
}

func TestEntriesFromDirResolvesOrgIDAgainstOrganizationsFile(t *testing.T) {
	dir := t.TempDir()
	// A real capture had no inline org name at all - only an id reference.
	writeJSON(t, dir, "entries-4.json", `{"success":true,"count":2,"data":[
		{"entryId":"1","organizationId":42},
		{"entryId":"2","organizationId":99}
	]}`)
	writeJSON(t, dir, "organizations.json", `{"success":true,"count":1,"data":[
		{"organizationId":42,"name":"Springfield High School","abbreviation":"SHS"}
	]}`)

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

// TestEntriesFromDirOnlyExtractsOrgsFromTheirOwnField is a regression test:
// organizations only ever come from bulk.json's Data.Organizations field or
// a dedicated organizations.json listing - both explicitly typed - so an
// unrelated object elsewhere in bulk.json (e.g. an event, which also has an
// id and could coincidentally collide with a real organization's id) can
// never be mistaken for one, structurally, unlike the old heuristic walk
// that had to guard against this by filename alone.
func TestEntriesFromDirOnlyExtractsOrgsFromTheirOwnField(t *testing.T) {
	dir := t.TempDir()
	// Event id "1" here deliberately collides with organization id "1" -
	// RC ids very likely restart at 1 per entity kind.
	writeJSON(t, dir, "bulk.json", `{"success":true,"count":1,"data":{
		"events":[{"eventId":1,"title":"Not An Organization","entries":[{"entryId":"1","organizationId":"1"}]}],
		"organizations":[{"organizationId":1,"name":"Springfield High School"}]
	}}`)

	entries, _, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if len(entries) != 1 || entries[0].OrgName != "Springfield High School" {
		t.Errorf("entries = %+v, want the entry resolved to the real org, not the event object", entries)
	}
}

func TestIsOrganizationsFileAndIsEventsFileAndIsBulkFile(t *testing.T) {
	if !isOrganizationsFile("organizations.json") || isOrganizationsFile("bulk.json") {
		t.Error("isOrganizationsFile misclassified a filename")
	}
	if !isEventsFile("events.json") {
		t.Error("isEventsFile should match events.json")
	}
	if isEventsFile("entries-4.json") {
		t.Error("isEventsFile must not match an entries-<id>.json file")
	}
	if !isBulkFile("bulk.json") || isBulkFile("entries-4.json") {
		t.Error("isBulkFile misclassified a filename")
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

// TestEntriesFromDirIgnoresUnrecognizedFileShapes covers files that are
// valid JSON but not one of the four recognized shapes (e.g. rcprobe's
// token.json, active-races.json) - these must be skipped, not treated as an
// error or as zero-value entries.
func TestEntriesFromDirIgnoresUnrecognizedFileShapes(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "token.json", `{"access_token":"not-a-real-token"}`)
	writeJSON(t, dir, "entries-1.json", `{"success":true,"count":1,"data":[{"entryId":"1","organizationId":"5"}]}`)

	entries, _, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "1" {
		t.Errorf("entries = %+v, want just the one real entry, token.json ignored", entries)
	}
}

// TestEntriesFromDirUsesEntrysOwnEventIDNotFilename is a regression test for
// the simplification this migration makes: EventID comes from the entry's
// own confirmed-real "eventId" field, not the capture's filename - so even a
// mismatched or missing filename hint doesn't matter anymore.
func TestEntriesFromDirUsesEntrysOwnEventIDNotFilename(t *testing.T) {
	dir := t.TempDir()
	// Deliberately mismatched: the filename says event 99, the entry's own
	// field says event 10.
	writeJSON(t, dir, "entries-99.json", `{"success":true,"count":1,"data":[{"entryId":"1","eventId":10}]}`)

	entries, _, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if len(entries) != 1 || entries[0].EventID != "10" {
		t.Errorf("entries = %+v, want EventID \"10\" from the entry's own field, not \"99\" from the filename", entries)
	}
}

func TestEntriesFromDirBuildsEventsIndex(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "events.json", `{"success":true,"count":2,"data":[
		{"eventId":10,"title":"Varsity 8"},
		{"eventId":11,"label":"Junior Varsity 8"}
	]}`)

	_, events, err := entriesFromDir(dir)
	if err != nil {
		t.Fatalf("entriesFromDir: %v", err)
	}
	if events["10"] != "Varsity 8" || events["11"] != "Junior Varsity 8" {
		t.Errorf("events = %v, want {10: Varsity 8, 11: Junior Varsity 8}", events)
	}
}

func TestEntryFromRCExtractsParticipantNames(t *testing.T) {
	e := entryFromRC(regattacentral.Entry{
		ID: "61",
		Participants: []regattacentral.Participant{
			{ID: "1", Name: "Alex Mihalovich"},
			{ID: "2", Name: "Jamie Smith"},
		},
	})
	want := []string{"Alex Mihalovich", "Jamie Smith"}
	if len(e.ParticipantNames) != len(want) || e.ParticipantNames[0] != want[0] || e.ParticipantNames[1] != want[1] {
		t.Errorf("ParticipantNames = %v, want %v", e.ParticipantNames, want)
	}
}

func TestEntryFromRCNoParticipantsLeavesParticipantNamesNil(t *testing.T) {
	e := entryFromRC(regattacentral.Entry{ID: "1"})
	if e.ParticipantNames != nil {
		t.Errorf("ParticipantNames = %v, want nil", e.ParticipantNames)
	}
}

func TestEntryFromRCBoatClassPrefersDivisionOverAlternateTitle(t *testing.T) {
	e := entryFromRC(regattacentral.Entry{ID: "1", Division: "Junior", AlternateTitle: "M-Jr-1x"})
	if e.BoatClass != "Junior" {
		t.Errorf("BoatClass = %q, want Division (\"Junior\") preferred over AlternateTitle", e.BoatClass)
	}
	e = entryFromRC(regattacentral.Entry{ID: "1", AlternateTitle: "M-Jr-1x"})
	if e.BoatClass != "M-Jr-1x" {
		t.Errorf("BoatClass = %q, want AlternateTitle used when Division is blank", e.BoatClass)
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
