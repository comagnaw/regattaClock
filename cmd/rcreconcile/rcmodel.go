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

	// EventID is which RC event this entry belongs to - read from its
	// capture's filename ("entries-<eventID>.json", exactly what `rcprobe
	// walk`/`entries` name them), not guessed from an in-document field.
	// Blank for entries found in a file that isn't named that way (e.g.
	// bulk.json), which just means event-scoping can't use it - see
	// entriesForRace.
	EventID string

	// OrgID is set when the entry references its organization by id rather
	// than carrying the name inline (confirmed the real shape: a real /bulk +
	// entries capture had no org name strings at all). Resolved against an
	// rcOrg index built from every file in the same --rc-dir - see
	// entriesFromDir.
	OrgID string

	OrgName      string
	OrgShortName string
	OrgAbbrev    string
	BoatClass    string

	// Label is a boat-label hint ("A"/"B" for a school's second boat), if the
	// /bulk payload carries one under any of the guessed field names below.
	// RegattaCentral may not track this distinction at all - it may be purely
	// a heat-sheet-authoring convention - so two same-event entries from one
	// school can legitimately stay ambiguous; see disambiguateByLabel.
	Label string
}

// rcOrg is what an rcEntry's OrgID resolves against - a RegattaCentral
// organization (club/school). Built from a dedicated organizations listing
// (rcprobe orgs / walk), not from an entry.
type rcOrg struct {
	ID        string
	Name      string
	ShortName string
	Abbrev    string
}

// entriesFromDir reads every *.json file directly inside dir - typically an
// rcprobe --out capture directory: bulk.json, organizations.json, events.json,
// and whatever entries-<eventID>.json files `rcprobe walk` wrote. For each
// file (one parse, not one per lookup kind) it collects entries, organizations
// and events, then: tags each entry found in an "entries-<id>.json" file with
// that id as EventID; merges entries (first occurrence of an ID wins, in
// sorted-filename order, so "bulk.json" sorts ahead of "entries-*.json" and is
// treated as the more authoritative source for name fields); and resolves any
// entry that only has an OrgID against the merged rcOrg index. Also returns
// the merged eventID -> label index for entriesForRace to scope pools by
// event.
//
// EventID is the one field exempt from "first occurrence wins": bulk.json
// nests the same entries the per-event entries-<id>.json files do (confirmed
// once asEntry started accepting an org-id-only reference - see Milestone
// 1.6), and bulk.json's copy never carries a filename-derived EventID. Without
// this exemption, bulk.json's blank EventID would win the merge for nearly
// every real entry, silently starving entriesForRace's event-scoping
// (bestMatchingEvent) of the one signal it depends on - reproducing the exact
// "every school shows N duplicate copies of itself as ambiguous" symptom
// event-scoping was built to fix. So a later duplicate's non-blank EventID
// always backfills an earlier, blank one.
func entriesFromDir(dir string) (entries []rcEntry, events map[string]string, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read %q: %w", dir, err)
	}

	var names []string
	for _, f := range files {
		if !f.IsDir() && strings.EqualFold(filepath.Ext(f.Name()), ".json") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)

	seenEntry := map[string]int{} // entry ID -> its index in merged
	orgs := map[string]rcOrg{}
	events = map[string]string{}
	var merged []rcEntry

	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read %q: %w", path, err)
		}

		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, nil, fmt.Errorf("%q: %w", path, err)
		}

		fileEventID := eventIDFromFilename(name)

		var found []rcEntry
		walkForEntries(v, &found)
		for _, e := range found {
			if fileEventID != "" {
				e.EventID = fileEventID
			}
			if idx, ok := seenEntry[e.ID]; ok {
				if merged[idx].EventID == "" && e.EventID != "" {
					merged[idx].EventID = e.EventID
				}
				continue
			}
			seenEntry[e.ID] = len(merged)
			merged = append(merged, e)
		}

		// Orgs and events are extracted only from their own dedicated listing
		// file, not from every file in the directory. RC ids very likely
		// restart at 1 per entity kind (an org, an event, and an entry can
		// all legitimately be id "1"), so walking bulk.json/entries-*.json for
		// asOrg/asEvent matches risks an unrelated object - another entity
		// kind that happens to have an "id" and a "name" field - colliding
		// with a real organization's or event's id and silently shadowing it.
		// organizations.json / events.json are each one coherent list from
		// one endpoint, so that collision can't happen within them.
		if isOrganizationsFile(name) {
			foundOrgs := map[string]rcOrg{}
			walkForOrgs(v, foundOrgs)
			for id, o := range foundOrgs {
				if _, ok := orgs[id]; !ok {
					orgs[id] = o
				}
			}
		}

		if isEventsFile(name) {
			foundEvents := map[string]string{}
			walkForEvents(v, foundEvents)
			for id, label := range foundEvents {
				if _, ok := events[id]; !ok {
					events[id] = label
				}
			}
		}
	}

	for i := range merged {
		if merged[i].OrgName == "" && merged[i].OrgID != "" {
			if o, ok := orgs[merged[i].OrgID]; ok {
				merged[i].OrgName = o.Name
				merged[i].OrgShortName = o.ShortName
				merged[i].OrgAbbrev = o.Abbrev
			}
		}
	}
	return merged, events, nil
}

// eventIDFromFilename returns the id in an "entries-<id>.json" filename, or ""
// if name doesn't match that pattern.
func eventIDFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	id, ok := strings.CutPrefix(base, "entries-")
	if !ok || id == "" {
		return ""
	}
	return id
}

// isOrganizationsFile reports whether name looks like the dedicated
// organizations listing rcprobe writes ("organizations.json", from `orgs` or
// `walk`).
func isOrganizationsFile(name string) bool {
	return strings.Contains(strings.ToLower(name), "organization")
}

// isEventsFile reports whether name looks like the dedicated events listing
// rcprobe's `events` command writes ("events.json") - deliberately excluding
// "entries-<id>.json" (one event's *entries*, not the events list) even
// though both names contain "event".
func isEventsFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "event") && !strings.HasPrefix(lower, "entries-")
}

// walkForEntries recurses through v (a parsed /bulk, entries, or similar
// response) collecting every Entry-shaped object it finds - see asEntry,
// which is where the PROVISIONAL, guessed field names actually live and where
// `rcreconcile shape`'s output should be used to correct them.
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

func walkForOrgs(v any, out map[string]rcOrg) {
	switch t := v.(type) {
	case map[string]any:
		if o, ok := asOrg(t); ok {
			out[o.ID] = o
			return
		}
		for _, child := range t {
			walkForOrgs(child, out)
		}
	case []any:
		for _, child := range t {
			walkForOrgs(child, out)
		}
	}
}

func walkForEvents(v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		if id, label, ok := asEvent(t); ok {
			out[id] = label
			return
		}
		for _, child := range t {
			walkForEvents(child, out)
		}
	case []any:
		for _, child := range t {
			walkForEvents(child, out)
		}
	}
}

// asEntry heuristically recognizes an Entry-shaped object. Field-name
// candidates are PROVISIONAL guesses; extend the candidate lists once
// `rcreconcile shape` shows the real names. An entry needs an id and either an
// inline organization name or a reference to one by id - a real capture had
// no inline org name at all, only an id reference resolved separately (see
// entriesFromDir).
func asEntry(m map[string]any) (rcEntry, bool) {
	id := firstString(m, "id", "entryId", "crewId")
	if id == "" {
		return rcEntry{}, false
	}
	org := firstOrgName(m)
	orgID := firstString(m, "organizationId", "orgId", "clubId", "teamId", "organisationId")
	if org == "" && orgID == "" {
		return rcEntry{}, false
	}
	return rcEntry{
		ID:           id,
		OrgID:        orgID,
		OrgName:      org,
		OrgShortName: firstString(m, "shortName", "orgShortName", "organizationShortName"),
		OrgAbbrev:    firstString(m, "abbreviation", "orgAbbreviation", "organizationAbbreviation"),
		BoatClass:    firstString(m, "boatClass", "equipmentType", "eventName", "className"),
		Label:        firstString(m, "label", "displayNumber", "boatLabel", "suffix"),
	}, true
}

// asOrg heuristically recognizes a plain Organization object: an id-like
// field plus its own name-like field. This is deliberately simpler than
// asEntry's org-name check (which looks for orgName/organizationName/a nested
// .organization.name - a reference shape) so the two do not fire on the same
// object: a plain organization's own name field is expected to just be
// "name". PROVISIONAL; extend once `rcreconcile shape` shows the real names.
func asOrg(m map[string]any) (rcOrg, bool) {
	id := firstString(m, "id", "organizationId", "orgId")
	name := firstString(m, "name")
	if id == "" || name == "" {
		return rcOrg{}, false
	}
	return rcOrg{
		ID:        id,
		Name:      name,
		ShortName: firstString(m, "shortName"),
		Abbrev:    firstString(m, "abbreviation"),
	}, true
}

// asEvent heuristically recognizes an Event-shaped object: an id plus a
// name/title/boat-class-like label. Used only to scope an entry's race by the
// event it belongs to (entriesForRace) - an entry's own boat class has never
// been reliably inline (see asEntry), and RC's shape has consistently turned
// out to be normalized/id-based rather than denormalized. PROVISIONAL; may
// collide with asOrg on a plain "name" field (both are approximate on
// purpose) - extend/narrow once `rcreconcile shape` (against events.json)
// shows the real names.
func asEvent(m map[string]any) (id, label string, ok bool) {
	id = firstString(m, "id", "eventId")
	label = firstString(m, "name", "title", "boatClass", "eventName", "className", "description")
	if id == "" || label == "" {
		return "", "", false
	}
	return id, label, true
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
