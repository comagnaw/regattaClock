package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

// runWalk is the `walk` command: one call that pulls /bulk and then follows it
// with a GET .../events/{id}/entries for every event id it can find in that
// response, saving all of it into outDir. It exists because it is unconfirmed
// whether /bulk nests full entries per event or just event/regatta metadata
// (see cmd/rcreconcile's rcmodel.go) - this gets whatever is missing in one
// command instead of the operator hand-running `entries <eventID>` once per
// event.
func runWalk(ctx context.Context, client *regattacentral.Client, regattaID, outDir string) error {
	if outDir == "" {
		return fmt.Errorf("walk: --out is required - it needs somewhere to save bulk.json and each event's entries")
	}

	raw, err := client.Bulk(ctx, regattaID)
	if err != nil {
		return fmt.Errorf("walk: fetch bulk: %w", err)
	}
	if err := saveQuiet(outDir, "bulk", raw); err != nil {
		return fmt.Errorf("walk: save bulk: %w", err)
	}

	ids, err := eventIDsFromBulk(raw)
	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}
	fmt.Fprintf(os.Stderr, "walk: found %d event id(s) in bulk\n", len(ids))
	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "walk: no event ids recognized in bulk.json - run "+
			"`rcreconcile shape --bulk-file "+outDir+"/bulk.json` to see its real structure "+
			"and widen eventIDsFromBulk / walkForEventIDs in cmd/rcprobe/walk.go")
		return nil
	}

	var failed []string
	for _, id := range ids {
		entries, err := client.EventEntries(ctx, regattaID, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "walk: event %s: %v\n", id, err)
			failed = append(failed, id)
			continue
		}
		if err := saveQuiet(outDir, "entries-"+id, entries); err != nil {
			fmt.Fprintf(os.Stderr, "walk: event %s: save: %v\n", id, err)
			failed = append(failed, id)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("walk: %d of %d event(s) failed: %s", len(failed), len(ids), strings.Join(failed, ", "))
	}
	return nil
}

// eventIDsFromBulk walks a /bulk (or similar) response looking for event ids,
// with no fixed path assumed - the same structural, no-fixed-schema approach
// as cmd/rcreconcile's entry walker, applied to events instead. Two signals,
// either anywhere in the document:
//
//   - a map with an "eventId" key (a back-reference, wherever it appears);
//   - a map with an "id" key, one level under a field whose name contains
//     "event" (case-insensitive) - e.g. an element of an "events" array.
//
// Returns unique ids in first-seen order.
func eventIDsFromBulk(raw json.RawMessage) ([]string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("decode bulk response: %w", err)
	}

	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	walkForEventIDs(v, "", add)
	return ids, nil
}

// walkForEventIDs recurses through v. parentKey is the field name (not a full
// path) whose value led to v, so the "id"-under-"event*" check only ever looks
// at the immediate container - a nested "entries" array does not inherit an
// ancestor "events" array's name.
func walkForEventIDs(v any, parentKey string, add func(string)) {
	switch t := v.(type) {
	case map[string]any:
		if id, ok := t["eventId"]; ok {
			add(scalarToString(id))
		} else if strings.Contains(strings.ToLower(parentKey), "event") {
			if id, ok := t["id"]; ok {
				add(scalarToString(id))
			}
		}
		for k, child := range t {
			walkForEventIDs(child, k, add)
		}
	case []any:
		for _, child := range t {
			walkForEventIDs(child, parentKey, add) // array elements inherit the array's own key
		}
	}
}

func scalarToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return ""
}

// saveQuiet writes raw to <outDir>/<name>.json without echoing it to stdout -
// unlike dump/emit, used by every other command, walk can fetch dozens of
// responses and dumping each to the terminal would bury the summary.
func saveQuiet(outDir, name string, raw json.RawMessage) error {
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		pretty = string(raw)
	}
	b, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(outDir, filesystem.SanitizeForFilename(name)+".json")
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "wrote", path)
	return nil
}
