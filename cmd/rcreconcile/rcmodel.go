package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// rcEntry is the handful of RegattaCentral /bulk fields the matcher needs.
// This is deliberately not internal/regattacentral's model: the exact /bulk
// shape is unconfirmed (see
// docs/features/personas/regattacentral-integration.md "What the schema tells
// us"), and this investigation is what confirms it - promoting a real model
// there is Milestone 4, once bulkEntries below has been corrected against
// real, observed field names via `rcreconcile shape`.
type rcEntry struct {
	// ID identifies the entry on RegattaCentral. Kept as a string since it is
	// unconfirmed whether RC ids are numeric or UUIDs.
	ID string

	OrgName      string
	OrgShortName string
	OrgAbbrev    string
	BoatClass    string

	// Label is a boat-label hint ("A"/"B" for a school's second boat), if the
	// /bulk payload carries one under any of the guessed field names below.
	Label string
}

// entriesFromDir reads every *.json file directly inside dir - typically an
// rcprobe --out capture directory, e.g. bulk.json plus whatever
// entries-<eventID>.json files `rcprobe walk` wrote - and merges the rcEntry
// values bulkEntries finds in each. Files are read in sorted-name order and
// the first occurrence of an ID wins, so "bulk.json" (sorted before
// "entries-*.json") is treated as the more authoritative source when the same
// entry appears in more than one capture.
func entriesFromDir(dir string) ([]rcEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", dir, err)
	}

	var names []string
	for _, f := range files {
		if !f.IsDir() && strings.EqualFold(filepath.Ext(f.Name()), ".json") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)

	seen := map[string]bool{}
	var merged []rcEntry
	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		found, err := bulkEntries(raw)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", path, err)
		}
		for _, e := range found {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			merged = append(merged, e)
		}
	}
	return merged, nil
}

// bulkEntries extracts a flat list of rcEntry from a raw /bulk response.
//
// PROVISIONAL: rather than assume one fixed path (e.g.
// "regattas[0].events[].entries[]"), this walks the whole document looking
// for objects that look like an Entry - something with an id-like field and
// an organization-name-like field - using guessed field names from the
// Cookbook's entity list and the LaneConstructor example. It is expected to
// need correction once the real shape is known: run
// `rcreconcile shape --bulk-file <path>` and use its (PII-free) key-path
// output to fix the field-name candidates below.
func bulkEntries(raw json.RawMessage) ([]rcEntry, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("decode bulk response: %w", err)
	}
	var found []rcEntry
	walkForEntries(v, &found)
	return found, nil
}

func walkForEntries(v any, out *[]rcEntry) {
	switch t := v.(type) {
	case map[string]any:
		if e, ok := asEntry(t); ok {
			*out = append(*out, e)
			// An Entry object is not expected to nest another Entry inside
			// itself, so do not recurse further into a match.
			return
		}
		for _, child := range t {
			walkForEntries(child, out)
		}
	case []any:
		for _, child := range t {
			walkForEntries(child, out)
		}
	}
}

// asEntry heuristically recognizes an Entry-shaped object. Field-name
// candidates are PROVISIONAL guesses; extend the candidate lists once
// `rcreconcile shape` shows the real names.
func asEntry(m map[string]any) (rcEntry, bool) {
	id := firstString(m, "id", "entryId", "crewId")
	org := firstOrgName(m)
	if id == "" || org == "" {
		return rcEntry{}, false
	}
	return rcEntry{
		ID:           id,
		OrgName:      org,
		OrgShortName: firstString(m, "shortName", "orgShortName", "organizationShortName"),
		OrgAbbrev:    firstString(m, "abbreviation", "orgAbbreviation", "organizationAbbreviation"),
		BoatClass:    firstString(m, "boatClass", "equipmentType", "eventName", "className"),
		Label:        firstString(m, "label", "displayNumber", "boatLabel", "suffix"),
	}, true
}

// firstOrgName tries a flat organization-name field first, then a nested
// "organization" object.
func firstOrgName(m map[string]any) string {
	if s := firstString(m, "orgName", "organizationName", "clubName", "teamName"); s != "" {
		return s
	}
	if org, ok := m["organization"].(map[string]any); ok {
		return firstString(org, "name", "shortName", "abbreviation")
	}
	return ""
}

// firstString returns the first non-blank string value among keys, converting
// a bare number to its decimal string (RC ids may be numeric in the JSON).
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				return s
			}
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
	}
	return ""
}
