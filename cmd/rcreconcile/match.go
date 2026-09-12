package main

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/comagnaw/regattaClock/internal/reader"
)

// matchStatus classifies how confidently one xlsm lane was matched to a
// RegattaCentral entry.
type matchStatus string

const (
	statusMatched   matchStatus = "matched"
	statusAmbiguous matchStatus = "ambiguous"
	statusUnmatched matchStatus = "unmatched"
	// statusGuessed is statusAmbiguous's last-resort sibling: every
	// narrowing signal (event scoping, label, rower name, boat class) was
	// tried and still left more than one candidate - genuinely
	// indistinguishable from the data available (typically two boats from
	// the same school in the same class). Only produced when matchRaces is
	// called with guessTies true (see --guess-ties); Candidates[0] is the
	// picked entry, with the untaken alternative(s) following it.
	statusGuessed matchStatus = "guessed"
)

// laneMatch is one xlsm lane and what it resolved to on RegattaCentral.
type laneMatch struct {
	RaceNumber     int
	Lane           int
	BoatClass      string
	FlightInfo     string
	SchoolName     string
	AdditionalInfo string
	// Place, Split and Time are the xlsm's post-race outcome for this lane
	// (reader.RaceEntry's own field names) - blank before the regatta has run,
	// or for a bye lane. Used by buildUploadPreview (preview.go) to fill in
	// ResultRecord/lane status; unused by the reconciliation report itself.
	Place  string
	Split  string
	Time   string
	Status matchStatus
	// Candidates is the matched entry for statusMatched, the competing entries
	// for statusAmbiguous, and empty for statusUnmatched.
	Candidates []rcEntry
}

// matchRaces compares every lane across races against entries and returns one
// laneMatch per lane (in race/lane order) plus the entries never referenced by
// any lane - a possible scratch, or a boat RC has that the lineup never
// included. events is the eventID -> label index from entriesFromDir, used to
// scope each race's candidate pool to its own RC event (see entriesForRace);
// pass nil to fall back to the plain boat-class filter. heatSheet is the
// (raceNumber, lane) -> heatSheetLane index from readHeatSheet
// (heatsheet.go), used only to disambiguate; pass nil if the xlsm has no
// Heat Sheet tab. guessTies opts into a last-resort tie-break (see
// statusGuessed) for whatever stays ambiguous after every other signal has
// been tried; false preserves the default, always-honest "ambiguous" status.
func matchRaces(races []reader.RaceData, entries []rcEntry, events map[string]string, heatSheet map[[2]int]heatSheetLane, guessTies bool) (matches []laneMatch, unused []rcEntry) {
	used := map[string]bool{}
	guessed := map[string]bool{} // entry ids already given to another tied lane, so ties never collide

	for _, race := range races {
		pool := entriesForRace(entries, race, events)
		for lane, entry := range race.OrderedLanes() {
			cands, _ := matchLane(race, lane, entry, pool, entries, events, heatSheet)
			lm := laneMatch{
				RaceNumber:     race.RaceNumber,
				Lane:           lane,
				BoatClass:      race.BoatClass,
				FlightInfo:     race.FlightInfo,
				SchoolName:     entry.SchoolName,
				AdditionalInfo: entry.AdditionalInfo,
				Place:          entry.Place,
				Split:          entry.Split,
				Time:           entry.Time,
				Candidates:     cands,
			}
			switch {
			case len(cands) == 0:
				lm.Status = statusUnmatched
			case len(cands) == 1:
				lm.Status = statusMatched
				used[cands[0].ID] = true
			case guessTies:
				if pick, ok := lastResortPick(cands, guessed); ok {
					lm.Status = statusGuessed
					lm.Candidates = reorderPickFirst(cands, pick)
					guessed[pick.ID] = true
					used[pick.ID] = true
				} else {
					// More tied lanes than distinct candidates left to give
					// them - reusing one would be an outright wrong
					// duplicate assignment, not just an imprecise guess, so
					// this lane stays honestly ambiguous instead.
					lm.Status = statusAmbiguous
				}
			default:
				lm.Status = statusAmbiguous
			}
			matches = append(matches, lm)
		}
	}

	for _, e := range entries {
		if !used[e.ID] {
			unused = append(unused, e)
		}
	}
	return matches, unused
}

// lastResortPick returns the first of cands not already in guessed, so two
// lanes that end up with the exact same tied candidate set (the real
// scenario this exists for - two boats from the same school in the same
// class, which RegattaCentral's own data doesn't distinguish) don't collide
// on the same entry. ok is false when every candidate is already spoken for.
func lastResortPick(cands []rcEntry, guessed map[string]bool) (pick rcEntry, ok bool) {
	for _, c := range cands {
		if !guessed[c.ID] {
			return c, true
		}
	}
	return rcEntry{}, false
}

// reorderPickFirst returns cands with pick moved to index 0 - so
// Candidates[0] is conventionally "the selection" (buildUploadPreview reads
// it that way for statusMatched already) while the untaken alternative(s)
// stay visible after it for the report's transparency.
func reorderPickFirst(cands []rcEntry, pick rcEntry) []rcEntry {
	out := make([]rcEntry, 0, len(cands))
	out = append(out, pick)
	for _, c := range cands {
		if c.ID != pick.ID {
			out = append(out, c)
		}
	}
	return out
}

// matchLane resolves one lane's candidates, in the same narrowing order
// matchRaces always applies, and also returns a step-by-step trace (school
// and organization names, entry ids and counts only - never an athlete's
// name) for --debug-race (see traceRace in reconcile.go). matchRaces ignores
// the trace in normal operation.
func matchLane(race reader.RaceData, lane int, entry reader.RaceEntry, pool, allEntries []rcEntry, events map[string]string, heatSheet map[[2]int]heatSheetLane) (cands []rcEntry, trace []string) {
	scoped := disambiguateByLabel(candidatesFor(entry.SchoolName, pool), entry.AdditionalInfo)
	trace = append(trace, fmt.Sprintf("lane %d (%s): race-scoped pool -> %d candidate(s) %s",
		lane, entry.SchoolName, len(scoped), candidateSummary(scoped)))

	if len(scoped) == 1 {
		return scoped, trace
	}

	// The race-scoped pool didn't resolve to exactly one candidate - either
	// nothing matched at all (e.g. the RD combined a different class into
	// this race's open lanes - heatsheet-rc-pivot-investigation.md), or it
	// matched more than one entry sharing this school's name within the
	// (possibly mis-resolved, or genuinely shared) scoped event - a real
	// example: a school raced three boats in one event but only two of its
	// RC entries actually belong there, so all three lanes' org-name match
	// returned the same wrong two. Retry against every entry in the regatta
	// and let the Heat Sheet's rower name / boat class try to narrow *that*
	// wider set - it is discarded below unless it does strictly better than
	// the scoped result, so this can never regress an already-good match.
	widened := disambiguateByLabel(candidatesFor(entry.SchoolName, allEntries), entry.AdditionalInfo)
	trace = append(trace, fmt.Sprintf("lane %d (%s): widened to full regatta -> %d candidate(s) %s",
		lane, entry.SchoolName, len(widened), candidateSummary(widened)))

	hs := heatSheet[[2]int{race.RaceNumber, lane}]
	resolved := widened
	if hs.RowerLastName != "" {
		before := len(resolved)
		resolved = disambiguateByRowerLastName(resolved, hs.RowerLastName)
		trace = append(trace, fmt.Sprintf("lane %d (%s): heat sheet rower name present -> narrowed %d to %d %s",
			lane, entry.SchoolName, before, len(resolved), candidateSummary(resolved)))
	} else {
		trace = append(trace, fmt.Sprintf("lane %d (%s): no heat sheet rower name for this lane", lane, entry.SchoolName))
	}

	if hs.LaneClass != "" {
		before := len(resolved)
		resolved = disambiguateByBoatClass(resolved, hs.LaneClass, events)
		trace = append(trace, fmt.Sprintf("lane %d (%s): heat sheet lane class %q present -> narrowed %d to %d %s",
			lane, entry.SchoolName, hs.LaneClass, before, len(resolved), candidateSummary(resolved)))
	} else {
		trace = append(trace, fmt.Sprintf("lane %d (%s): no heat sheet lane class for this lane", lane, entry.SchoolName))
	}

	switch {
	case len(resolved) == 1:
		return resolved, trace
	case len(scoped) > 0:
		// Widening + narrowing didn't do better than the scoped pool - keep
		// reporting the scoped, already-bounded candidate set rather than a
		// possibly much larger regatta-wide list that no signal could
		// actually narrow.
		return scoped, trace
	default:
		return resolved, trace
	}
}

// traceRace prints, to w, matchLane's step-by-step trace for every lane in
// race - school/org names, entry ids and counts only, never an athlete's
// name - so a specific lane's non-match can be diagnosed against a real
// capture without exposing anything sensitive. See --debug-race.
func traceRace(w io.Writer, race reader.RaceData, entries []rcEntry, events map[string]string, heatSheet map[[2]int]heatSheetLane) {
	pool := entriesForRace(entries, race, events)
	fmt.Fprintf(w, "[debug] race %d: nominal boat class=%q, race-scoped pool size=%d\n", race.RaceNumber, race.BoatClass, len(pool))
	for lane, entry := range race.OrderedLanes() {
		_, trace := matchLane(race, lane, entry, pool, entries, events, heatSheet)
		for _, line := range trace {
			fmt.Fprintln(w, "[debug] "+line)
		}
	}
}

// candidateSummary renders cands as entry ids and organization names only -
// never an athlete's name - for --debug-race output.
func candidateSummary(cands []rcEntry) string {
	if len(cands) == 0 {
		return "[]"
	}
	parts := make([]string, len(cands))
	for i, c := range cands {
		org := c.OrgName
		if org == "" {
			org = "unknown org"
		}
		parts[i] = fmt.Sprintf("id=%s org=%q event=%s class=%q", c.ID, org, c.EventID, c.BoatClass)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// entriesForRace scopes the candidate pool to one RC event's entries, tried in
// order of how much can be trusted:
//
//  1. Roster overlap (bestMatchingEvent): which event's entries best match the
//     schools actually racing in this race. This is the primary mechanism -
//     see bestMatchingEvent for why.
//  2. Event label text (matchingEventIDs against events, the eventID -> label
//     index entriesFromDir builds from events.json et al.) - kept as a second
//     opinion in case a regatta's labels do line up with the xlsm's
//     BoatClass/FlightInfo text, which real data has not shown so far.
//  3. entriesForBoatClass (today's plain filter) - the final fallback whenever
//     no event can be resolved for this race at all.
//
// Every step can only narrow the pool further than the one before it produced
// nothing usable; this never regresses to fewer matches than before
// event-scoping existed.
func entriesForRace(entries []rcEntry, race reader.RaceData, events map[string]string) []rcEntry {
	if id := bestMatchingEvent(entries, race); id != "" {
		if pool := entriesWithEventID(entries, id); len(pool) > 0 {
			return pool
		}
	}

	if ids := matchingEventIDs(race, events); len(ids) > 0 {
		var pool []rcEntry
		for _, e := range entries {
			if e.EventID != "" && ids[e.EventID] {
				pool = append(pool, e)
			}
		}
		if len(pool) > 0 {
			return pool
		}
	}

	return entriesForBoatClass(entries, race.BoatClass)
}

// bestMatchingEvent finds which RC event's entries best match the schools
// actually racing in race, by counting - per event - how many *distinct*
// schools in race's lineup resolve (via the same comparison candidatesFor
// uses) to an entry tagged with that event. The event id is returned only if
// it has a clear lead over every other event's count; a tie (including 0-0)
// returns "" rather than guess.
//
// This is preferred over matching event labels to the xlsm's BoatClass/
// FlightInfo text (matchingEventIDs): a real regatta's xlsm used short codes
// like "M-2-4+" / "W-Jr-4x" that share no useful text with RegattaCentral's
// fuller event names, so label matching resolved nothing and every race fell
// back to the full, unscoped entry pool - every school with more than one
// boat in the whole regatta then showed up as "ambiguous" in every race it
// raced in, not just its own. Roster overlap sidesteps needing the two sides'
// text to agree on anything: it uses data both sides already agree on, which
// schools are racing together.
func bestMatchingEvent(entries []rcEntry, race reader.RaceData) string {
	raceSchools := map[string]bool{}
	for _, lane := range race.Lanes {
		if n := normalize(lane.SchoolName); n != "" {
			raceSchools[n] = true
		}
	}
	if len(raceSchools) == 0 {
		return ""
	}

	counts := map[string]int{}
	for school := range raceSchools {
		matchedEvents := map[string]bool{}
		for _, e := range entries {
			if e.EventID == "" || e.OrgName == "" {
				continue
			}
			if matchOrgName(school, e) != orgNone {
				matchedEvents[e.EventID] = true
			}
		}
		for id := range matchedEvents {
			counts[id]++
		}
	}

	var bestID string
	var bestCount, secondCount int
	for id, c := range counts {
		switch {
		case c > bestCount:
			bestID, bestCount, secondCount = id, c, bestCount
		case c > secondCount:
			secondCount = c
		}
	}
	if bestCount == 0 || bestCount == secondCount {
		return ""
	}
	return bestID
}

func entriesWithEventID(entries []rcEntry, eventID string) []rcEntry {
	var out []rcEntry
	for _, e := range entries {
		if e.EventID == eventID {
			out = append(out, e)
		}
	}
	return out
}

// matchingEventIDs returns the ids of every event in events whose label
// exactly equals race's (normalized) BoatClass or FlightInfo. See
// entriesForRace: this is a secondary check behind bestMatchingEvent.
//
// Deliberately exact, not substring-either-way like candidatesFor's org-name
// match: boat classes are drawn from a small, systematically-prefixed
// vocabulary ("Varsity 8", "Junior Varsity 8", "Novice Varsity 8", ...) where
// a shorter class name is routinely a literal substring of a longer,
// different one - "varsity8" is a substring of "juniorvarsity8" - so
// substring matching here would silently merge distinct boat classes instead
// of separating them.
func matchingEventIDs(race reader.RaceData, events map[string]string) map[string]bool {
	out := map[string]bool{}
	for _, field := range []string{race.BoatClass, race.FlightInfo} {
		n := normalize(field)
		if n == "" {
			continue
		}
		for id, label := range events {
			if normalize(label) == n {
				out[id] = true
			}
		}
	}
	return out
}

// entriesForBoatClass narrows the candidate pool to entries whose boat class
// matches race's, when that filtering actually narrows anything - RC's
// BoatClass field is one of the least certain (see rcmodel.go), so an entry
// with no recognized boat class is kept in every pool rather than dropped.
func entriesForBoatClass(entries []rcEntry, boatClass string) []rcEntry {
	bc := normalize(boatClass)
	if bc == "" {
		return entries
	}
	var out []rcEntry
	for _, e := range entries {
		eb := normalize(e.BoatClass)
		if eb == "" || eb == bc {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return entries
	}
	return out
}

// candidatesFor finds pool entries whose organization name/short
// name/abbreviation matches school. An exact normalized match on any of the
// three name fields wins outright; otherwise a substring match on either side
// is kept as a weaker candidate set.
func candidatesFor(school string, pool []rcEntry) []rcEntry {
	n := normalize(school)
	if n == "" {
		return nil
	}
	var exact, partial []rcEntry
	for _, e := range pool {
		switch matchOrgName(n, e) {
		case orgExact:
			exact = append(exact, e)
		case orgPartial:
			partial = append(partial, e)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return partial
}

type orgMatchKind int

const (
	orgNone orgMatchKind = iota
	orgPartial
	orgExact
)

// minSubstringMatchLen is the shortest a normalized name may be to
// participate in the substring side of matchOrgName. Below this length a
// name is only ever an exact match. Real example that motivated this: "Osbourn
// Park"'s abbreviation "OP" is a substring of "bishOPireton" ("Bishop
// Ireton") purely by coincidence - short strings are too likely to appear
// inside an unrelated longer one for "contains" to mean "probably the same
// school." This trades a few legitimate short-abbreviation matches (still
// reachable via an exact match) for far fewer coincidental false ones; ambiguous
// or unmatched is the safe direction to err in, since a human reviews both.
const minSubstringMatchLen = 4

func matchOrgName(normalizedSchool string, e rcEntry) orgMatchKind {
	best := orgNone
	for _, name := range []string{e.OrgName, e.OrgShortName, e.OrgAbbrev} {
		nn := normalize(name)
		if nn == "" {
			continue
		}
		if nn == normalizedSchool {
			return orgExact
		}
		if len(nn) >= minSubstringMatchLen && len(normalizedSchool) >= minSubstringMatchLen &&
			(strings.Contains(nn, normalizedSchool) || strings.Contains(normalizedSchool, nn)) {
			best = orgPartial
		}
	}
	return best
}

// commonAbbrevExpansions is a narrow, named list of rowing/school suffix
// abbreviations normalize expands before comparing, so "Springfield HS" and
// "Springfield High School" compare equal. This is not general NLP - anything
// outside this list is left unresolved (ambiguous/unmatched) for a human to
// judge, which is the point: the report flags what it is unsure of rather
// than guessing silently.
var commonAbbrevExpansions = map[string]string{
	"hs": "highschool",
	"ms": "middleschool",
	"jv": "juniorvarsity",
	"rc": "rowingclub",
	"bc": "boatclub",
	"st": "saint",
}

// disambiguateByLabel narrows more-than-one candidate down to one using a
// boat label ("A"/"B") parsed from the xlsm lane's AdditionalInfo, matched
// against rcEntry.Label. Leaves cands untouched if that narrows to anything
// other than exactly one.
func disambiguateByLabel(cands []rcEntry, additionalInfo string) []rcEntry {
	if len(cands) <= 1 {
		return cands
	}
	label := extractBoatLabel(additionalInfo)
	if label == "" {
		return cands
	}
	var narrowed []rcEntry
	for _, c := range cands {
		if strings.EqualFold(strings.TrimSpace(c.Label), label) {
			narrowed = append(narrowed, c)
		}
	}
	if len(narrowed) == 1 {
		return narrowed
	}
	return cands
}

// disambiguateByRowerLastName narrows more-than-one candidate down to one
// using a rower's last name from the xlsm's Heat Sheet tab (listed only for
// 1x/2x boats - see readHeatSheet, heatsheet.go). Matches when the name
// appears as a whole token in any of the entry's participant names
// (normalized) - PROVISIONAL like every other name-shape guess in this tool,
// since the real API's name format ("First Last" vs "Last, First", etc.) is
// unconfirmed. A blank lastName, or anything that isn't really a name (an
// advancement note, "Exhibition", "SCRATCHED"), simply won't match any
// participant and leaves cands untouched, same as disambiguateByLabel.
func disambiguateByRowerLastName(cands []rcEntry, lastName string) []rcEntry {
	if len(cands) <= 1 {
		return cands
	}
	want := tokens(lastName)
	if len(want) == 0 {
		return cands
	}
	var narrowed []rcEntry
	for _, c := range cands {
		if entryHasParticipantToken(c, want) {
			narrowed = append(narrowed, c)
		}
	}
	if len(narrowed) == 1 {
		return narrowed
	}
	return cands
}

// disambiguateByBoatClass narrows more-than-one candidate down to one using
// the Heat Sheet tab's row-2 per-lane text (heatSheetLane.LaneClass) - set
// when the RD combines a different class into this race's open lanes (see
// readHeatSheet), but that same cell can just as easily hold "A"/"B",
// "SCRATCHED", or an advancement note, which is why this only narrows on an
// EXACT normalized match (unlike disambiguateByRowerLastName's whole-token
// check) - a short boat-class code is too easy to coincidentally
// half-match another one, the same risk matchingEventIDs guards against for
// event labels. Tries each candidate's own BoatClass field first, then its
// resolved event's label (via events) - PROVISIONAL like every other
// field-shape guess in this tool.
func disambiguateByBoatClass(cands []rcEntry, laneClass string, events map[string]string) []rcEntry {
	if len(cands) <= 1 {
		return cands
	}
	n := normalizeBoatClass(laneClass)
	if n == "" {
		return cands
	}
	var narrowed []rcEntry
	for _, c := range cands {
		if normalizeBoatClass(c.BoatClass) == n || normalizeBoatClass(events[c.EventID]) == n {
			narrowed = append(narrowed, c)
		}
	}
	if len(narrowed) == 1 {
		return narrowed
	}
	return cands
}

// heatSheetClassDecorators is a short, named list of local RD annotations
// that can share a Heat Sheet cell with a real boat class (e.g. a real
// example: "Exhibition M-1-4x") but never appear in RegattaCentral's own
// class/event-label text - stripped before comparing so a decorated lane can
// still exact-match. Same "named list, not general NLP" approach as
// commonAbbrevExpansions; a decorator outside this list is left unresolved
// for a human to judge, same as everything else this tool is unsure of.
var heatSheetClassDecorators = map[string]bool{
	"exhibition": true,
}

// normalizeBoatClass is normalize's boat-class-specific sibling: same
// tokenize-and-join approach, but strips heatSheetClassDecorators instead of
// expanding commonAbbrevExpansions (a decorator word isn't a synonym for
// part of the class, it's an unrelated annotation riding along in the same
// cell). Deliberately still exact-match-only where it's used
// (disambiguateByBoatClass) - a short class code is too easy to
// coincidentally substring-match a different one (e.g. "Varsity 8" inside
// "Junior Varsity 8"), the same risk matchingEventIDs guards against.
func normalizeBoatClass(s string) string {
	var b strings.Builder
	for _, t := range tokens(s) {
		if heatSheetClassDecorators[t] {
			continue
		}
		b.WriteString(t)
	}
	return b.String()
}

func entryHasParticipantToken(e rcEntry, want []string) bool {
	for _, name := range e.ParticipantNames {
		for _, t := range tokens(name) {
			if slices.Contains(want, t) {
				return true
			}
		}
	}
	return false
}

var boatLabelRe = regexp.MustCompile(`(?i)\b([A-Z])\b`)

func extractBoatLabel(s string) string {
	m := boatLabelRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return strings.ToUpper(m[1])
}

// normalize lower-cases s, splits it into alphanumeric tokens, expands any
// token found in commonAbbrevExpansions, and joins the result with no
// separator - so "Springfield HS", "springfield high school" and
// "SPRINGFIELD, High-School" all compare equal.
func normalize(s string) string {
	var b strings.Builder
	for _, t := range tokens(s) {
		if exp, ok := commonAbbrevExpansions[t]; ok {
			b.WriteString(exp)
		} else {
			b.WriteString(t)
		}
	}
	return b.String()
}

// tokens splits s into lower-cased runs of letters/digits, discarding
// everything else as a separator.
func tokens(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
