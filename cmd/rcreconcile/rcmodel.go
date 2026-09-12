package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

// rcEntry is the handful of RegattaCentral fields the matcher needs, mapped
// from the confirmed typed regattacentral.Entry (see entryFromRC) - kept as
// its own type rather than using regattacentral.Entry directly because
// OrgName/OrgShortName/OrgAbbrev are an rcreconcile-side join result (an
// entry only references its organization by id), not something RC's own
// Entry type carries inline.
type rcEntry struct {
	// ID identifies the entry on RegattaCentral. Kept as a string since RC
	// sends ids as either a bare JSON number or a quoted string (see
	// regattacentral.FlexibleID).
	ID string

	// EventID is which RC event this entry belongs to, read directly from
	// the entry's own confirmed-real "eventId" field (see
	// regattacentral.Entry) - no longer derived from the capture's filename,
	// now that every real entry has been observed to carry this field
	// inline and agree with itself across every file it appears in.
	EventID string

	// OrgID is the entry's organization reference (RC's Entry has no inline
	// org name - see entryFromRC). Resolved against an rcOrg index built
	// from every organizations-shaped file in the same --rc-dir.
	OrgID string

	OrgName      string
	OrgShortName string
	OrgAbbrev    string
	// BoatClass prefers the confirmed-real Division field, falling back to
	// AlternateTitle - see entryFromRC.
	BoatClass string

	// Label is the confirmed-real "entryLabel" field ("A"/"B" for a school's
	// second boat, when RC tracks it at all - it may be purely a
	// heat-sheet-authoring convention with no RC equivalent, so two
	// same-event entries from one school can legitimately stay ambiguous;
	// see disambiguateByLabel).
	Label string

	// ParticipantNames are the crew's rower/athlete names, from the entry's
	// confirmed-real "entryParticipants" array. Used only by
	// disambiguateByRowerLastName, matching against the Heat Sheet tab's
	// stroke-name column (see heatsheet.go) - never surfaced in the
	// reconciliation report itself.
	ParticipantNames []string
}

// rcOrg is what an rcEntry's OrgID resolves against - a RegattaCentral
// organization (club/school), mapped from the confirmed typed
// regattacentral.Organization.
type rcOrg struct {
	ID        string
	Name      string
	ShortName string
	Abbrev    string
}

// entriesFromDir reads every *.json file directly inside dir - typically an
// rcprobe --out capture directory: bulk.json, organizations.json, events.json,
// and whatever entries-<eventID>.json files `rcprobe walk` wrote. Each file
// is decoded into its confirmed regattacentral typed shape (DecodeBulk for
// bulk.json's nested events/entries, DecodeOrganizations/DecodeEvents for
// their dedicated listings, DecodeEntries for everything else) rather than
// the field-name-guessing structural walk this file used before those types
// existed - see docs/features/personas/heatsheet-rc-pivot-investigation.md's
// Milestone 4 entry for why. A file that doesn't decode as an entries
// listing (e.g. token.json, active-races.json) is skipped rather than
// erroring the whole read, the same tolerance the old heuristic walk had for
// files that simply didn't match any recognized shape.
//
// Entries are deduped by ID, first occurrence wins (sorted-filename order):
// the same real entry appearing in both bulk.json and its own
// entries-<id>.json carries identical field values either way, so unlike the
// old heuristic walk there is no special-case merging needed for EventID or
// any other field - confirmed empirically (zero blank EventID/OrganizationID
// across every entry in a real 107-entry capture). Also returns the merged
// eventID -> label index for entriesForRace to scope pools by event.
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

	seenEntry := map[string]bool{}
	orgs := map[string]rcOrg{}
	events = map[string]string{}
	var merged []rcEntry

	addEntry := func(e rcEntry) {
		if e.ID == "" || seenEntry[e.ID] {
			return
		}
		seenEntry[e.ID] = true
		merged = append(merged, e)
	}
	addOrg := func(o rcOrg) {
		if o.ID == "" {
			return
		}
		if _, ok := orgs[o.ID]; !ok {
			orgs[o.ID] = o
		}
	}
	addEventLabel := func(id, label string) {
		if id == "" || label == "" {
			return
		}
		if _, ok := events[id]; !ok {
			events[id] = label
		}
	}

	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read %q: %w", path, err)
		}

		switch {
		case isBulkFile(name):
			bulk, err := regattacentral.DecodeBulk(raw)
			if err != nil {
				return nil, nil, fmt.Errorf("%q: %w", path, err)
			}
			for _, ev := range bulk.Data.Events {
				addEventLabel(string(ev.ID), eventLabel(ev))
				for _, e := range ev.Entries {
					addEntry(entryFromRC(e))
				}
			}
			for _, o := range bulk.Data.Organizations {
				addOrg(orgFromRC(o))
			}

		case isOrganizationsFile(name):
			resp, err := regattacentral.DecodeOrganizations(raw)
			if err != nil {
				return nil, nil, fmt.Errorf("%q: %w", path, err)
			}
			for _, o := range resp.Data {
				addOrg(orgFromRC(o))
			}

		case isEventsFile(name):
			resp, err := regattacentral.DecodeEvents(raw)
			if err != nil {
				return nil, nil, fmt.Errorf("%q: %w", path, err)
			}
			for _, ev := range resp.Data {
				addEventLabel(string(ev.ID), eventLabel(ev))
			}

		default:
			resp, err := regattacentral.DecodeEntries(raw)
			if err != nil {
				// Not every *.json file in --rc-dir is an entries listing
				// (token.json, active-races.json, ...) - skip rather than
				// failing the whole read.
				continue
			}
			// Defensive cross-check only, never fatal: every real entry
			// observed so far already carries its own correct eventId, so
			// this should never fire - if it does, the inline field can no
			// longer be trusted blindly and needs another look.
			if fileEventID := eventIDFromFilename(name); fileEventID != "" {
				for _, e := range resp.Data {
					if id := string(e.EventID); id != "" && id != fileEventID {
						fmt.Fprintf(os.Stderr, "rcreconcile: %s: entry %s's eventId (%s) disagrees with the filename (%s)\n",
							name, e.ID, id, fileEventID)
					}
				}
			}
			for _, e := range resp.Data {
				addEntry(entryFromRC(e))
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

// entryFromRC maps a confirmed regattacentral.Entry into the matcher's own
// rcEntry shape. BoatClass prefers Division, falling back to AlternateTitle
// - both confirmed-real fields (traced from a live entry's own JSON keys),
// replacing the old asEntry's unconfirmed boatClass/equipmentType/className
// guesses entirely.
func entryFromRC(e regattacentral.Entry) rcEntry {
	var names []string
	for _, p := range e.Participants {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return rcEntry{
		ID:               string(e.ID),
		EventID:          string(e.EventID),
		OrgID:            string(e.OrganizationID),
		BoatClass:        firstNonBlank(e.Division, e.AlternateTitle),
		Label:            e.Label,
		ParticipantNames: names,
	}
}

func orgFromRC(o regattacentral.Organization) rcOrg {
	return rcOrg{
		ID:        string(o.ID),
		Name:      o.Name,
		ShortName: o.ShortName,
		Abbrev:    o.Abbreviation,
	}
}

// eventLabel picks the best display label for an event - Title over Label,
// both confirmed-real fields; empty if neither is set.
func eventLabel(ev regattacentral.Event) string {
	return firstNonBlank(ev.Title, ev.Label)
}

func firstNonBlank(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// eventIDFromFilename returns the id in an "entries-<id>.json" filename, or ""
// if name doesn't match that pattern. Used only as a defensive cross-check
// against each entry's own confirmed-real eventId field (see
// entriesFromDir) - no longer the primary source of EventID.
func eventIDFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	id, ok := strings.CutPrefix(base, "entries-")
	if !ok || id == "" {
		return ""
	}
	return id
}

// isBulkFile reports whether name looks like the combined /bulk capture
// rcprobe writes ("bulk.json", from `bulk` or `walk`).
func isBulkFile(name string) bool {
	return strings.Contains(strings.ToLower(name), "bulk")
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
