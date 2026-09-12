package main

import (
	"regexp"
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
// pass nil to fall back to the plain boat-class filter.
func matchRaces(races []reader.RaceData, entries []rcEntry, events map[string]string) (matches []laneMatch, unused []rcEntry) {
	used := map[string]bool{}

	for _, race := range races {
		pool := entriesForRace(entries, race, events)
		for lane, entry := range race.OrderedLanes() {
			cands := disambiguateByLabel(candidatesFor(entry.SchoolName, pool), entry.AdditionalInfo)
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
			switch len(cands) {
			case 0:
				lm.Status = statusUnmatched
			case 1:
				lm.Status = statusMatched
				used[cands[0].ID] = true
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
