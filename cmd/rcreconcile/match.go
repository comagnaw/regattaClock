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
	Status         matchStatus
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

// entriesForRace scopes the candidate pool to entries tagged with an EventID
// whose event label (from events, the eventID -> label index entriesFromDir
// builds from events.json et al.) matches race's BoatClass/FlightInfo. An
// entry's own boat class has never turned out to be reliably inline - see
// asEntry - so this is preferred over entriesForBoatClass, which it falls
// back to whenever no event can be resolved for this race, or resolves to
// events none of the entries are tagged with (e.g. all entries came from
// bulk.json, which carries no filename-derived EventID). The fallback means
// this can only narrow a pool, never regress to fewer matches than before
// event-scoping existed.
func entriesForRace(entries []rcEntry, race reader.RaceData, events map[string]string) []rcEntry {
	ids := matchingEventIDs(race, events)
	if len(ids) == 0 {
		return entriesForBoatClass(entries, race.BoatClass)
	}
	var out []rcEntry
	for _, e := range entries {
		if e.EventID != "" && ids[e.EventID] {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return entriesForBoatClass(entries, race.BoatClass)
	}
	return out
}

// matchingEventIDs returns the ids of every event in events whose label
// exactly equals race's (normalized) BoatClass or FlightInfo.
//
// Deliberately exact, not substring-either-way like candidatesFor's org-name
// match: boat classes are drawn from a small, systematically-prefixed
// vocabulary ("Varsity 8", "Junior Varsity 8", "Novice Varsity 8", ...) where
// a shorter class name is routinely a literal substring of a longer,
// different one - "varsity8" is a substring of "juniorvarsity8" - so
// substring matching here would silently merge distinct boat classes instead
// of separating them. An unresolved race falls back to entriesForBoatClass
// (see entriesForRace), so exact-only just means "narrow when confident,
// otherwise don't guess."
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
		if strings.Contains(nn, normalizedSchool) || strings.Contains(normalizedSchool, nn) {
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
