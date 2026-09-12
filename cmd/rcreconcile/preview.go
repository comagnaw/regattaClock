package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

// buildUploadPreview is Milestone 2: adapt matches (Milestone 1's
// reconciliation) into a regattacentral.UploadRequest shaped exactly like the
// payload Client.Upload would PUT to /regattas/{id}/upload (Cookbook §10-15).
// Nothing in cmd/rcreconcile calls Client.Upload - this is a local dry-run
// only, written to a file by writeUploadPreview.
//
// A confidently matched lane gets its real, already-confirmed RC EntryID.
// Anything not confidently matched (ambiguous or unmatched) gets a
// locally-generated placeholder UUID instead - the same "assign a UUID for an
// entry this client created locally" shape Cookbook §11 documents for a
// brand-new entry, reused here purely as a stand-in - and is called out in
// the returned warnings so the preview's reader knows which lanes are
// guesses, not confirmed RegattaCentral data. A result is only added for a
// lane with a parseable finish time (see parseRaceTime): a race that hasn't
// happened yet, or a bye lane, still gets a lane record but no result.
func buildUploadPreview(matches []laneMatch) (*regattacentral.UploadRequest, []string) {
	req := &regattacentral.UploadRequest{}
	var warnings []string
	hasResult := map[int]bool{}

	for _, m := range matches {
		// DisplayNumber reuses extractBoatLabel (built for the "A"/"B" a
		// multi-boat school gets), which is PROVISIONAL here too:
		// reader.RaceEntry.AdditionalInfo can instead hold a small boat's
		// rower name (see its doc comment), so a name with a lone capital
		// letter can produce a false "label" - acceptable for a preview
		// that is never uploaded, but not something to trust blindly.
		lane := regattacentral.LaneRecord{
			RaceNumber:    m.RaceNumber,
			Lane:          m.Lane,
			DisplayNumber: extractBoatLabel(m.AdditionalInfo),
			Status:        laneStatus(m.Place, m.LaneClass),
		}

		switch m.Status {
		case statusMatched, statusGuessed:
			id, err := strconv.Atoi(m.Candidates[0].ID)
			if err != nil {
				lane.UUID = newPlaceholderUUID()
				warnings = append(warnings, fmt.Sprintf(
					"Race %d Lane %d (%s): RegattaCentral entry id %q isn't numeric - using a placeholder id instead of the real one",
					m.RaceNumber, m.Lane, m.SchoolName, m.Candidates[0].ID))
			} else {
				lane.EntryID = id
			}
			if m.Status == statusGuessed {
				warnings = append(warnings, fmt.Sprintf(
					"Race %d Lane %d (%s): --guess-ties picked RegattaCentral entry id %s over an equally plausible alternative - not a confident match",
					m.RaceNumber, m.Lane, m.SchoolName, m.Candidates[0].ID))
			}
		case statusAmbiguous:
			lane.UUID = newPlaceholderUUID()
			warnings = append(warnings, fmt.Sprintf(
				"Race %d Lane %d (%s): more than one possible RegattaCentral entry - using a placeholder id until a human picks the right one",
				m.RaceNumber, m.Lane, m.SchoolName))
		default: // statusUnmatched
			lane.UUID = newPlaceholderUUID()
			warnings = append(warnings, fmt.Sprintf(
				"Race %d Lane %d (%s): no RegattaCentral entry found - would be created as new",
				m.RaceNumber, m.Lane, m.SchoolName))
		}

		req.AddLane(lane)

		if d, ok := parseRaceTime(m.Time); ok {
			req.AddFinish(m.RaceNumber, m.Lane, d)
			hasResult[m.RaceNumber] = true
		}
	}

	for raceNumber := range hasResult {
		req.SetRaceStatus(raceNumber, regattacentral.StatusOfficial)
	}

	return req, warnings
}

// laneStatus combines the xlsm's post-race Place outcome with the Heat
// Sheet's pre-race "Exhibition" designation (see isExhibitionLane, match.go)
// into one regattacentral.LaneStatus. A real, observed outcome takes
// priority - DQ/DNF/DNS/SCR are hard facts recorded at the finish line (or a
// pre-race scratch), whereas Exhibition is an administrative designation
// made before the race even starts; if the lane finished cleanly with no
// outcome-level status, its Exhibition flag (if any) is what's reported.
func laneStatus(place, laneClass string) regattacentral.LaneStatus {
	if s := laneStatusForPlace(place); s != regattacentral.LaneOK {
		return s
	}
	if isExhibitionLane(laneClass) {
		return regattacentral.LaneExhibition
	}
	return regattacentral.LaneOK
}

// laneStatusForPlace maps the xlsm's Place field to a RegattaCentral
// LaneStatus. common.RaceDisqualification/DidNotFinish/DidNotStart ("DQ" /
// "DNF" / "DNS") are the values internal/clock's live timing UI writes there
// (see laps.go's isNonPlace); "SCR"/"SCRATCHED" are not part of that app
// vocabulary - a scratch is known before the race even starts, at the
// Heat Sheet stage (see heatsheet.go), not something the finish-line clock
// marks - but an RD can still hand-type either into the post-race Results
// tab for a boat that never rowed, so both are recognized here too. Anything
// else - a real finishing place, or blank for a race that hasn't happened -
// reports as LaneOK.
func laneStatusForPlace(place string) regattacentral.LaneStatus {
	switch strings.ToUpper(strings.TrimSpace(place)) {
	case common.RaceDisqualification:
		return regattacentral.LaneDisqualified
	case common.RaceDidNotFinish:
		return regattacentral.LaneDidNotFinish
	case common.RaceDidNotStart:
		return regattacentral.LaneDidNotStart
	case "SCR", "SCRATCHED":
		return regattacentral.LaneScratched
	default:
		return regattacentral.LaneOK
	}
}

// parseRaceTime parses the xlsm Results tab's time format, "M:SS.s" (see
// reader.RaceEntry.Time, e.g. "6:12.5"), into a time.Duration. Returns
// ok=false for blank or unparseable input - DQ/DNF/DNS live in Place, not
// Time, so a lane with one of those still reaches here with a blank Time and
// correctly gets no result record.
func parseRaceTime(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	minutes, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(minutes)*time.Minute + time.Duration(seconds*float64(time.Second)), true
}

// newPlaceholderUUID generates a random RFC 4122 v4-shaped id to stand in for
// a lane's real RegattaCentral EntryID when none is confidently known. It is
// never sent anywhere - this preview is local-only - so it only needs to look
// like the kind of id Cookbook §11 expects for a client-created entry.
func newPlaceholderUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// writeUploadPreview writes path as one local, human-readable file: a
// per-race/lane rendering first, then the raw JSON payload req would carry to
// PUT /regattas/{id}/upload - never sent; Client.Upload is not called
// anywhere in cmd/rcreconcile. Like --report-out, this names real people once
// run against a real capture - keep it outside the repo or under a gitignored
// path (see README.md's PII warning).
func writeUploadPreview(path string, req *regattacentral.UploadRequest, warnings []string) error {
	if err := req.Validate(false); err != nil {
		return fmt.Errorf("upload preview would be invalid: %w", err)
	}

	var b strings.Builder
	fmt.Fprintln(&b, "RegattaCentral upload preview - NOT sent, local dry-run only")
	fmt.Fprintln(&b, strings.Repeat("=", 60))
	fmt.Fprintln(&b)

	resultByKey := map[[2]int]regattacentral.ResultRecord{}
	for _, r := range req.Results {
		resultByKey[[2]int{r.RaceNumber, r.Lane}] = r
	}

	lanes := append([]regattacentral.LaneRecord(nil), req.Lanes...)
	sort.Slice(lanes, func(i, j int) bool {
		if lanes[i].RaceNumber != lanes[j].RaceNumber {
			return lanes[i].RaceNumber < lanes[j].RaceNumber
		}
		return lanes[i].Lane < lanes[j].Lane
	})

	for _, l := range lanes {
		id := "would create new entry (" + l.UUID + ")"
		if l.EntryID != 0 {
			id = fmt.Sprintf("RC entry #%d", l.EntryID)
		}
		line := fmt.Sprintf("Race %d Lane %d - %s", l.RaceNumber, l.Lane, id)
		if r, ok := resultByKey[[2]int{l.RaceNumber, l.Lane}]; ok {
			line += fmt.Sprintf(" - %s", time.Duration(r.Time)*time.Millisecond)
		}
		if l.Status != regattacentral.LaneOK {
			line += fmt.Sprintf(" [%s]", l.Status)
		}
		fmt.Fprintln(&b, line)
	}

	if len(warnings) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Needs a human's judgment before this could really be uploaded:")
		for _, w := range warnings {
			fmt.Fprintln(&b, "  - "+w)
		}
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, strings.Repeat("=", 60))
	fmt.Fprintln(&b, "Raw payload (what Client.Upload would PUT to /regattas/{id}/upload - never actually called):")
	payload, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal upload preview: %w", err)
	}
	b.Write(payload)
	b.WriteByte('\n')

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
