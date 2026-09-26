package ingest

import (
	"bufio"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed words/*.txt
var wordFiles embed.FS

// wordList reads one embedded list: a name per line, '#' comments and blank
// lines skipped.
func wordList(name string) []string {
	f, err := wordFiles.Open("words/" + name)
	if err != nil {
		panic(err) // embedded at build time; a missing file is a build bug
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	if err := sc.Err(); err != nil {
		panic(err)
	}
	return out
}

// designator is a trailing boat designator kept on a school name - "Potomac
// A", "Fairfax 2V" - so crews-per-school stays realistic.
var designator = regexp.MustCompile(`^([A-H]|[1-9]V|JV|FV)$`)

// splitSchool separates a school name from its trailing boat designator.
func splitSchool(name string) (base, des string) {
	fields := strings.Fields(name)
	if len(fields) > 1 && designator.MatchString(fields[len(fields)-1]) {
		return strings.Join(fields[:len(fields)-1], " "), fields[len(fields)-1]
	}
	return strings.Join(fields, " "), ""
}

// names returns the names in a rower or additional-info cell: its
// "/"-separated pieces ("Smith/Jones"), less any piece that is non-name
// vocabulary ("Exhibition", "W-Jr-2x", a bow number).
func names(cell string) []string {
	var out []string
	for p := range strings.SplitSeq(cell, "/") {
		if p = strings.TrimSpace(p); p != "" && !isNonName(p) {
			out = append(out, p)
		}
	}
	return out
}

// keepWords are AdditionalInfo tokens that are not names: flight and
// progression vocabulary, lane-status codes, and boat descriptors.
var keepWords = map[string]bool{
	"heat": true, "final": true, "finals": true, "flight": true, "exhibition": true, "exh": true,
	"scratched": true, "scratch": true, "scr": true, "rep": true, "repechage": true,
	"semi": true, "semifinal": true, "petite": true, "grand": true, "time": true, "trial": true,
	"to": true, "advance": true, "advances": true, "top": true, "next": true, "best": true,
	"and": true, "of": true, "the": true, "novice": true, "varsity": true, "jv": true, "fv": true,
	"lwt": true, "ltwt": true, "lightweight": true, "masters": true, "open": true, "boys": true,
	"girls": true, "men": true, "women": true, "mens": true, "womens": true, "coxed": true,
	"dns": true, "dnf": true, "dq": true, "bye": true, "tbd": true, "lane": true, "race": true,
	"event": true, "composite": true, "comp": true, "mixed": true,
}

// codeToken matches a token that cannot be a name: a bow or place number
// ("3", "2V", "3rd"), a lone letter, punctuation, or a boat-class code
// ("W-Jr-2x", "M-1-8+", "2x", "8+").
var codeToken = regexp.MustCompile(`^(\d+[A-Za-z+]{0,2}|[A-Za-z]|[A-Za-z]?\d+|[+#&-]|[MWXmwx](-[A-Za-z0-9+]+)+)$`)

// isNonName reports whether every token of s is known non-name vocabulary -
// bow numbers, "Heat 3", "Exhibition", "SCR", "M-2x Exhibition", ... An empty
// value is non-name too.
func isNonName(s string) bool {
	for _, tok := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '(' || r == ')'
	}) {
		if !codeToken.MatchString(tok) && !keepWords[strings.ToLower(tok)] {
			return false
		}
	}
	return true
}

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// obfuscator maps every real school and rower name to a realistic fake, one
// map for the whole day. A fake is chosen by sha256(seed, kind, name) into
// the embedded word lists, probing forward past fakes already in use and any
// that would contain a real name, so the result is deterministic for a seed
// and the order names are assigned in (always sorted).
type obfuscator struct {
	seed  int64
	leaks *leakSet

	places, suffixes, surnames []string

	schools map[string]string // normalized real base name -> fake base name
	rowers  map[string]string // normalized real rower name -> fake
	used    map[string]bool   // normalized fakes handed out
}

func newObfuscator(seed int64, leaks *leakSet) *obfuscator {
	return &obfuscator{
		seed:     seed,
		leaks:    leaks,
		places:   wordList("places.txt"),
		suffixes: wordList("suffixes.txt"),
		surnames: wordList("surnames.txt"),
		schools:  map[string]string{},
		rowers:   map[string]string{},
		used:     map[string]bool{},
	}
}

// assign gives every name a fake, schools then rowers, each in sorted order.
//
// Each kind draws from tiers, the realistic one first: a school gets a place
// stem no other school has, with a suffix; a rower a plain surname. Only
// when a tier is used up does a name fall to the next - any stem × suffix,
// then hyphenated surname pairs - so even a very large day gets distinct
// fakes.
func (o *obfuscator) assign(schoolBases, rowerNames []string) error {
	sort.Strings(schoolBases)
	sort.Strings(rowerNames)

	np, ns := len(o.places), len(o.suffixes)
	for _, real := range schoolBases {
		key := normalize(real)
		if _, ok := o.schools[key]; ok {
			continue
		}
		suffix := o.suffixes[o.hash("suffix", key)%ns]
		fake, err := o.choose("school", key,
			tier{np, func(i int) (string, string) {
				return o.places[i] + " " + suffix, "stem:" + normalize(o.places[i])
			}},
			tier{np * ns, func(i int) (string, string) {
				c := o.places[i/ns] + " " + o.suffixes[i%ns]
				return c, normalize(c)
			}})
		if err != nil {
			return err
		}
		o.schools[key] = fake
	}

	n := len(o.surnames)
	for _, real := range rowerNames {
		key := normalize(real)
		if _, ok := o.rowers[key]; ok {
			continue
		}
		fake, err := o.choose("rower", key,
			tier{n, func(i int) (string, string) {
				return o.surnames[i], normalize(o.surnames[i])
			}},
			tier{n * n, func(i int) (string, string) {
				c := o.surnames[i/n] + "-" + o.surnames[i%n]
				return c, normalize(c)
			}})
		if err != nil {
			return err
		}
		o.rowers[key] = fake
	}
	return nil
}

// tier is one candidate space: n candidates, each a fake plus the key that
// claims it (a school's stem, so no two schools share one; otherwise the
// fake itself).
type tier struct {
	n    int
	cand func(i int) (fake, claim string)
}

// choose picks key's fake from the first tier with a free candidate,
// starting at a seeded hash of key and probing forward past claimed
// candidates and any that would contain a real name.
func (o *obfuscator) choose(kind, key string, tiers ...tier) (string, error) {
	start := o.hash(kind, key)
	for _, t := range tiers {
		for k := range t.n {
			fake, claim := t.cand((start + k) % t.n)
			if o.used[claim] || o.used[normalize(fake)] || o.leaks.contains(fake) {
				continue
			}
			o.used[claim] = true
			o.used[normalize(fake)] = true
			return fake, nil
		}
	}
	return "", fmt.Errorf("ran out of fake %s names", kind)
}

// hash is a seeded, non-negative hash of kind and key.
func (o *obfuscator) hash(kind, key string) int {
	h := sha256.Sum256(fmt.Appendf(nil, "%d\x00%s\x00%s", o.seed, kind, key))
	return int(binary.BigEndian.Uint64(h[:8]) >> 1)
}

// school returns name's fake, its boat designator kept.
func (o *obfuscator) school(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	base, des := splitSchool(name)
	fake := o.schools[normalize(base)]
	if des != "" {
		fake += " " + des
	}
	return fake
}

// cell maps a rower or additional-info cell piece by piece: a "/"-separated
// piece that is non-name vocabulary is kept as it is, and any other is a
// rower name (some workbooks put a single's rower in additional info).
func (o *obfuscator) cell(s string) string {
	if isNonName(s) {
		return s
	}
	var out []string
	for p := range strings.SplitSeq(s, "/") {
		p = strings.TrimSpace(p)
		switch {
		case p == "":
		case isNonName(p):
			out = append(out, p)
		default:
			out = append(out, o.rowers[normalize(p)])
		}
	}
	return strings.Join(out, "/")
}

// leakSet is every real name the ingest saw. A name (or a distinctive word of
// one) of four or more letters matches anywhere in a string, case-insensitive;
// a shorter one ("Lee", "Ng") only as a whole word, so it cannot trip on an
// unrelated fake like "Leesburg".
type leakSet struct {
	long  map[string]bool
	short map[string]bool
}

// genericWords appear in real and fake school names alike - they identify no
// one, and leaking on them would reject every fake.
var genericWords = map[string]bool{
	"high": true, "school": true, "academy": true, "crew": true, "rowing": true, "club": true,
	"prep": true, "preparatory": true, "catholic": true, "country": true, "day": true,
	"collegiate": true, "boat": true, "association": true, "saint": true, "the": true,
	"and": true, "of": true, "county": true, "north": true, "south": true, "east": true, "west": true,
}

func newLeakSet() *leakSet {
	return &leakSet{long: map[string]bool{}, short: map[string]bool{}}
}

func (l *leakSet) add(name string) {
	n := normalize(name)
	if n == "" {
		return
	}
	if len(n) >= 4 {
		l.long[n] = true
	} else if len(n) >= 2 {
		l.short[n] = true
	}
	for _, w := range words(n) {
		if genericWords[w] {
			continue
		}
		if len(w) >= 4 {
			l.long[w] = true
		} else if len(w) >= 2 && len(strings.Fields(n)) == 1 {
			l.short[w] = true
		}
	}
}

// match returns the real name s contains, or "".
func (l *leakSet) match(s string) string {
	n := normalize(s)
	if n == "" {
		return ""
	}
	for r := range l.long {
		if strings.Contains(n, r) {
			return r
		}
	}
	for _, w := range words(n) {
		if l.short[w] {
			return w
		}
	}
	return ""
}

func (l *leakSet) contains(s string) bool { return l.match(s) != "" }

// words splits on anything but letters, digits and apostrophes.
func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '\'' || r > 127)
	})
}
