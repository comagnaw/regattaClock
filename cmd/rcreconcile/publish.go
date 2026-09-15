// publish-schedule and publish-results are cmd/rcreconcile's one deliberate
// exception to "never calls RegattaCentral's write API": built only after
// the RD explicitly approved publishing this one real, completed regatta's
// schedule and results for real - see
// docs/features/personas/heatsheet-rc-pivot-investigation.md. shape and
// reconcile remain permanently read-only.
//
// Both commands reuse reconcile's exact matching pipeline
// (entriesFromDir/readHeatSheet/matchRaces with --guess-ties always on) so
// the write path can never disagree with what reconcile already showed the
// operator. Only a lane with a confidently resolved, numeric RC EntryID
// (statusMatched or statusGuessed) is ever included - unlike the local
// upload preview (Milestone 2), a lane with no resolvable entry is left out
// entirely rather than given a placeholder UUID, since inventing a new RC
// registration on a live regatta was never asked for.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/personacfg"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/regattacentral"
	"github.com/comagnaw/regattaClock/internal/secretstore"
)

type publishOptions struct {
	xlsmPath, rcDir, regattaID, configFile, secretsFile, origin string
	confirm, dryRun                                             bool
}

func parsePublishFlags(name string, argv []string) (publishOptions, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var o publishOptions
	fs.StringVar(&o.xlsmPath, "xlsm", "", "path to the regatta's .xlsm/.xlsx workbook (required) - must be the same file used for reconcile")
	fs.StringVar(&o.rcDir, "rc-dir", "", "directory of captured RegattaCentral JSON files (required) - must be the same directory used for reconcile")
	fs.StringVar(&o.regattaID, "regatta", "", "the RegattaCentral regatta id to publish to (required)")
	fs.StringVar(&o.configFile, "config", "", "path to a personacfg deployment file for the API base URL")
	fs.StringVar(&o.secretsFile, "secrets-file", "", "path to a JSON secrets file (default: RC_* environment variables)")
	fs.StringVar(&o.origin, "origin", "", "Origin header to send - RegattaCentral requires it for a client id with a registered referer (see cmd/rcprobe's --origin); PROVISIONAL whether a write needs it when reads don't")
	fs.BoolVar(&o.confirm, "confirm", false, "actually write to RegattaCentral after the interactive confirmation prompt; without it, only the dry-run summary prints")
	fs.BoolVar(&o.dryRun, "dry-run", false, "print the exact URL and JSON body of every call --confirm would make (race creation, lane assignment, the /upload PUT), then exit - never touches the network, and overrides --confirm if both are given")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rcreconcile %s --xlsm PATH --rc-dir DIR --regatta ID [--config PATH] [--secrets-file PATH] [--origin URL] [--confirm] [--dry-run]\n\nflags:\n", name)
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		return o, err
	}
	if o.xlsmPath == "" || o.rcDir == "" || o.regattaID == "" {
		fs.Usage()
		return o, fmt.Errorf("%s: --xlsm, --rc-dir and --regatta are all required", name)
	}
	return o, nil
}

// resolveBaseURL applies the same --config resolution publishClient uses for
// regattacentral.Config.BaseURL, but returns a concrete, always-non-empty,
// always-trailing-slash URL (mirroring Config.applyDefaults, which is
// unexported) - shared so dry-run output and the real client agree on
// exactly which URL a call would go to.
func resolveBaseURL(o publishOptions) (string, error) {
	baseURL := ""
	if o.configFile != "" {
		cfg, err := personacfg.Load(o.configFile)
		if err != nil {
			return "", err
		}
		if rc, ok := cfg.RegattaCentralConfig(); ok {
			baseURL = rc.BaseURL
		}
	}
	if baseURL == "" {
		baseURL = regattacentral.DefaultBaseURL
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return baseURL, nil
}

// publishClient builds a regattacentral.Client, mirroring cmd/rcprobe's own
// credential/config wiring exactly (see cmd/rcprobe/main.go's newClient).
func publishClient(o publishOptions) (*regattacentral.Client, error) {
	if o.secretsFile != "" {
		secretstore.SetBackend(secretstore.NewFileBackend(o.secretsFile))
	} else {
		secretstore.SetBackend(secretstore.NewEnvBackend("RC_"))
	}
	creds, err := regattacentral.LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("%w\n(set RC_CLIENT_ID / RC_CLIENT_SECRET / RC_USERNAME / RC_PASSWORD, or pass --secrets-file)", err)
	}

	baseURL, err := resolveBaseURL(o)
	if err != nil {
		return nil, err
	}

	return regattacentral.New(regattacentral.Config{
		Credentials: creds,
		RegattaID:   o.regattaID,
		BaseURL:     baseURL,
		Origin:      o.origin,
		Timeout:     30 * time.Second,
	})
}

// classifyForPublish splits reconcile's matches into publishable
// (statusMatched/statusGuessed with a numeric EntryID AND a numeric EventID -
// the only lanes a live push ever sends), guessed (the subset of publishable
// that only resolved via --guess-ties, called out separately so the operator
// sees exactly which lanes are a best guess, not a confident match), and
// excluded (no resolvable entry, still ambiguous even after --guess-ties, or
// a non-numeric id this tool won't invent a placeholder for on a real write -
// see the package doc comment). A non-numeric EventID is excluded rather than
// defaulted, unlike the local-only upload preview: a wrong EventID risks
// writing into some other real event's race schedule, which is worse than a
// wrong/placeholder EntryID.
func classifyForPublish(matches []laneMatch) (publishable, guessed, excluded []laneMatch) {
	for _, m := range matches {
		if m.Status != statusMatched && m.Status != statusGuessed {
			excluded = append(excluded, m)
			continue
		}
		if _, err := strconv.Atoi(m.Candidates[0].ID); err != nil {
			excluded = append(excluded, m)
			continue
		}
		if _, err := strconv.Atoi(m.Candidates[0].EventID); err != nil {
			excluded = append(excluded, m)
			continue
		}
		publishable = append(publishable, m)
		if m.Status == statusGuessed {
			guessed = append(guessed, m)
		}
	}
	return publishable, guessed, excluded
}

// resolvedLanes runs reconcile's exact matching pipeline - guessTies is
// always true here, per the author's decision that a live push should
// include --guess-ties picks - and classifies the result for publish.
func resolvedLanes(o publishOptions) (rd *reader.RegattaData, publishable, guessed, excluded []laneMatch, err error) {
	rd, err = reader.ReadExcelFile(o.xlsmPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read xlsm %q: %w", o.xlsmPath, err)
	}
	entries, events, err := entriesFromDir(o.rcDir)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	heatSheet, _, err := readHeatSheet(o.xlsmPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read heat sheet: %w", err)
	}

	matches, _ := matchRaces(rd.SortedRaces(), entries, events, heatSheet, true)
	publishable, guessed, excluded = classifyForPublish(matches)
	return rd, publishable, guessed, excluded, nil
}

// raceKey identifies one (eventID, raceNumber) pair - a race whose
// publishable lanes span multiple RC events (the mixed-boat-class case)
// produces multiple raceKeys sharing raceNumber, one per event.
type raceKey struct{ eventID, raceNumber int }

// raceIDsFileName is written into --rc-dir by publish-schedule right after
// Client.CreateRaces succeeds, and read back by publish-results - both
// commands already require the same --rc-dir, so this needs no new flag.
// Kept as a plain file, not part of the gitignored capture set it lives
// alongside, for the same reason those are gitignored: it's specific to one
// real regatta's live publish run.
const raceIDsFileName = "race-ids.json"

// raceIDRecord is one race-ids.json entry: which (event, xlsm race number)
// pair maps to which real RegattaCentral race id, once Client.CreateRaces
// has assigned one.
type raceIDRecord struct {
	EventID    int `json:"eventId"`
	RaceNumber int `json:"raceNumber"`
	RaceID     int `json:"raceId"`
}

func raceIDsPath(rcDir string) string {
	return filepath.Join(rcDir, raceIDsFileName)
}

// loadRaceIDs reads a previously-written race-ids.json, if any. A missing
// file is not an error - it just means publish-schedule hasn't created any
// races yet.
func loadRaceIDs(rcDir string) (map[raceKey]int, error) {
	out := map[raceKey]int{}
	data, err := os.ReadFile(raceIDsPath(rcDir))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", raceIDsFileName, err)
	}
	var records []raceIDRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parse %s: %w", raceIDsFileName, err)
	}
	for _, r := range records {
		out[raceKey{r.EventID, r.RaceNumber}] = r.RaceID
	}
	return out, nil
}

// saveRaceIDs writes raceIDs back to race-ids.json, sorted for stable diffs
// across runs.
func saveRaceIDs(rcDir string, raceIDs map[raceKey]int) error {
	records := make([]raceIDRecord, 0, len(raceIDs))
	for k, id := range raceIDs {
		records = append(records, raceIDRecord{EventID: k.eventID, RaceNumber: k.raceNumber, RaceID: id})
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].EventID != records[j].EventID {
			return records[i].EventID < records[j].EventID
		}
		return records[i].RaceNumber < records[j].RaceNumber
	})
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", raceIDsFileName, err)
	}
	if err := os.WriteFile(raceIDsPath(rcDir), data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", raceIDsFileName, err)
	}
	return nil
}

// lanesByRaceKey groups publishable's lanes by (eventID, xlsm raceNumber),
// building each regattacentral.LaneRecord exactly as buildScheduleRequest
// does - shared so the creation step (createRaces) assigns the same lanes
// to a newly-created race that the /upload call will also send.
func lanesByRaceKey(publishable []laneMatch) map[raceKey][]regattacentral.LaneRecord {
	out := map[raceKey][]regattacentral.LaneRecord{}
	for _, m := range publishable {
		entryID, _ := strconv.Atoi(m.Candidates[0].ID)
		eventID, _ := strconv.Atoi(m.Candidates[0].EventID)
		k := raceKey{eventID, m.RaceNumber}
		out[k] = append(out[k], regattacentral.LaneRecord{
			Lane:          m.Lane,
			DisplayNumber: extractBoatLabel(m.AdditionalInfo),
			EntryID:       entryID,
			Status:        laneStatus(m.Place, m.LaneClass),
		})
	}
	return out
}

// missingRaceIDs returns the sorted, de-duplicated (eventID, raceNumber)
// pairs in publishable that raceIDs has no entry for - races publish-results
// cannot reference yet because publish-schedule hasn't created them.
func missingRaceIDs(publishable []laneMatch, raceIDs map[raceKey]int) []raceKey {
	seen := map[raceKey]bool{}
	var missing []raceKey
	for _, m := range publishable {
		eventID, _ := strconv.Atoi(m.Candidates[0].EventID)
		k := raceKey{eventID, m.RaceNumber}
		if seen[k] {
			continue
		}
		seen[k] = true
		if _, ok := raceIDs[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].eventID != missing[j].eventID {
			return missing[i].eventID < missing[j].eventID
		}
		return missing[i].raceNumber < missing[j].raceNumber
	})
	return missing
}

// createRaces creates, on RegattaCentral, every race referenced by
// publishable that isn't already recorded in raceIDs (from an earlier
// publish-schedule run), then assigns its lanes - see
// Client.CreateRaces/AssignLanes. Returns an updated copy of raceIDs (never
// mutates the map passed in) with the newly-created races merged in. A race
// already present in raceIDs is skipped entirely, so re-running
// publish-schedule against the same regatta does not create duplicates.
//
// RaceRecord.DisplayNumber (set to the xlsm race number as a string)
// correlates each request race to its response, since CreateRaces's
// response is not guaranteed to preserve request order.
func createRaces(ctx context.Context, client *regattacentral.Client, regattaID string, publishable []laneMatch, raceIDs map[raceKey]int) (map[raceKey]int, error) {
	lanes := lanesByRaceKey(publishable)

	updated := make(map[raceKey]int, len(raceIDs))
	maps.Copy(updated, raceIDs)

	newByEvent := map[int][]int{} // eventID -> xlsm race numbers not yet created
	seen := map[raceKey]bool{}
	for k := range lanes {
		if seen[k] {
			continue
		}
		seen[k] = true
		if _, ok := updated[k]; ok {
			continue
		}
		newByEvent[k.eventID] = append(newByEvent[k.eventID], k.raceNumber)
	}

	var eventIDs []int
	for eventID := range newByEvent {
		eventIDs = append(eventIDs, eventID)
	}
	sort.Ints(eventIDs)

	for _, eventID := range eventIDs {
		raceNumbers := newByEvent[eventID]
		sort.Ints(raceNumbers)

		req := make([]regattacentral.RaceRecord, len(raceNumbers))
		byDisplay := make(map[string]int, len(raceNumbers))
		for i, rn := range raceNumbers {
			dn := strconv.Itoa(rn)
			req[i] = regattacentral.RaceRecord{UUID: newPlaceholderUUID(), DisplayNumber: dn}
			byDisplay[dn] = rn
		}

		created, err := client.CreateRaces(ctx, regattaID, strconv.Itoa(eventID), req)
		if err != nil {
			return updated, fmt.Errorf("create races for event %d: %w", eventID, err)
		}
		if len(created) != len(raceNumbers) {
			return updated, fmt.Errorf("create races for event %d: requested %d, RegattaCentral returned %d", eventID, len(raceNumbers), len(created))
		}
		for _, race := range created {
			rn, ok := byDisplay[race.DisplayNumber]
			if !ok {
				return updated, fmt.Errorf("create races for event %d: response race with displayNumber %q doesn't match any requested race", eventID, race.DisplayNumber)
			}
			updated[raceKey{eventID, rn}] = race.RaceID
			if _, err := client.AssignLanes(ctx, regattaID, race.RaceID, lanes[raceKey{eventID, rn}]); err != nil {
				return updated, fmt.Errorf("assign lanes for race %d (event %d, xlsm race %d): %w", race.RaceID, eventID, rn, err)
			}
		}
	}
	return updated, nil
}

// buildScheduleRequest builds the publish-schedule payload: one LaneRecord
// per publishable lane, nested under a StatusDraw RaceRecord for every
// (event, race) pair that has one. SCR/EXH (via laneStatus) are lane-level
// facts already fully known from the historical xlsm, so including them at
// this stage - even though it's nominally "schedule, not results" - is
// correct. A race whose lanes resolve to more than one EventID is split:
// each event gets its own RaceRecord, holding only its own lanes (see
// regattacentral.RaceRecord's doc comment).
//
// raceIDs must already have a real RegattaCentral race id for every
// (eventID, raceNumber) pair in publishable - see createRaces, which
// populates it before this is ever called. RaceRecord.RaceID is the real
// id, never the xlsm race number - Upload's own Validate() rejects a zero
// RaceID, so a caller bug that skips race creation fails loudly here rather
// than silently referencing a race that doesn't exist.
func buildScheduleRequest(publishable []laneMatch, raceIDs map[raceKey]int) *regattacentral.UploadRequest {
	req := &regattacentral.UploadRequest{}
	for k, ls := range lanesByRaceKey(publishable) {
		raceID := raceIDs[k]
		for _, l := range ls {
			req.AddLane(k.eventID, raceID, l)
		}
		req.SetRaceStatus(k.eventID, raceID, strconv.Itoa(k.raceNumber), regattacentral.StatusDraw)
	}
	return req
}

// buildResultsRequest builds the publish-results payload: for every
// publishable lane with a parseable finish time, the FULL lane record (not
// just the result) nested under a StatusOfficial RaceRecord. The full lane
// is resent, not just the result, because RC's upload semantics overwrite a
// race record wholesale on each PUT (Cookbook §13) - sending a bare
// {lane, results} with entryId/status omitted risks blanking what
// publish-schedule already set. See buildScheduleRequest for raceIDs.
func buildResultsRequest(publishable []laneMatch, raceIDs map[raceKey]int) *regattacentral.UploadRequest {
	req := &regattacentral.UploadRequest{}
	races := map[raceKey]bool{}
	for _, m := range publishable {
		d, ok := parseRaceTime(m.Time)
		if !ok {
			continue
		}
		entryID, _ := strconv.Atoi(m.Candidates[0].ID)
		eventID, _ := strconv.Atoi(m.Candidates[0].EventID)
		k := raceKey{eventID, m.RaceNumber}
		raceID := raceIDs[k]
		req.AddLane(eventID, raceID, regattacentral.LaneRecord{
			Lane:          m.Lane,
			DisplayNumber: extractBoatLabel(m.AdditionalInfo),
			EntryID:       entryID,
			Status:        laneStatus(m.Place, m.LaneClass),
		})
		req.AddFinish(eventID, raceID, m.Lane, d)
		races[k] = true
	}
	for k := range races {
		req.SetRaceStatus(k.eventID, raceIDs[k], strconv.Itoa(k.raceNumber), regattacentral.StatusOfficial)
	}
	return req
}

// splitRaceNumbers returns, for every race number in publishable whose lanes
// resolve to more than one distinct EventID, the sorted distinct EventIDs
// involved - the mixed-boat-class scenario that gets emitted as multiple
// RaceRecords sharing one RaceID (see buildScheduleRequest). This behavior is
// this tool's best inference from available evidence, not something
// RegattaCentral's docs explicitly confirm - callers surface it to the
// operator before any real write.
func splitRaceNumbers(publishable []laneMatch) map[int][]string {
	events := map[int]map[string]bool{}
	for _, m := range publishable {
		if events[m.RaceNumber] == nil {
			events[m.RaceNumber] = map[string]bool{}
		}
		events[m.RaceNumber][m.Candidates[0].EventID] = true
	}
	out := map[int][]string{}
	for raceNumber, set := range events {
		if len(set) <= 1 {
			continue
		}
		var ids []string
		for id := range set {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out[raceNumber] = ids
	}
	return out
}

func totalLaneCount(rd *reader.RegattaData) int {
	n := 0
	for _, race := range rd.SortedRaces() {
		n += len(race.Lanes)
	}
	return n
}

// printSummaryAndConfirm prints the dry-run summary to out (always) and,
// only when o.confirm is set, prompts on in for the regatta id back before
// returning true. Never prints an athlete's name - school/organization
// names and RC entry ids only, the same PII boundary every other diagnostic
// in this tool already follows.
func printSummaryAndConfirm(out io.Writer, in io.Reader, stageLabel string, o publishOptions, rd *reader.RegattaData, publishable, guessed, excluded []laneMatch) bool {
	fmt.Fprintf(out, "About to PUBLISH to RegattaCentral regatta %s - this is a live write, not a preview.\n", o.regattaID)
	fmt.Fprintf(out, "Stage: %s\n", stageLabel)
	races := map[int]bool{}
	for _, m := range publishable {
		races[m.RaceNumber] = true
	}
	fmt.Fprintf(out, "  %d race(s), %d lane(s) total\n", len(races), totalLaneCount(rd))
	fmt.Fprintf(out, "  %d lane(s) confidently matched\n", len(publishable)-len(guessed))

	if len(guessed) > 0 {
		fmt.Fprintf(out, "  %d lane(s) resolved only via --guess-ties (best guess, not confirmed):\n", len(guessed))
		for _, m := range sortedByRaceLane(guessed) {
			fmt.Fprintf(out, "    Race %d Lane %d (%s) -> RC entry %s\n", m.RaceNumber, m.Lane, m.SchoolName, m.Candidates[0].ID)
		}
	}
	if len(excluded) > 0 {
		fmt.Fprintf(out, "  %d lane(s) NOT published - no resolvable RC entry, handle manually on RC's site:\n", len(excluded))
		for _, m := range sortedByRaceLane(excluded) {
			fmt.Fprintf(out, "    Race %d Lane %d (%s)\n", m.RaceNumber, m.Lane, m.SchoolName)
		}
	}

	if split := splitRaceNumbers(publishable); len(split) > 0 {
		fmt.Fprintf(out, "  %d race(s) SPLIT ACROSS MULTIPLE RC EVENTS - each will be uploaded as more than one nested race record sharing the same raceId (inferred behavior, not confirmed against RegattaCentral):\n", len(split))
		var raceNumbers []int
		for rn := range split {
			raceNumbers = append(raceNumbers, rn)
		}
		sort.Ints(raceNumbers)
		for _, rn := range raceNumbers {
			fmt.Fprintf(out, "    Race %d: RC events %s\n", rn, strings.Join(split[rn], ", "))
		}
	}

	if !o.confirm {
		fmt.Fprintln(out, "(--confirm not set - nothing was written. Re-run with --confirm to proceed.)")
		return false
	}

	fmt.Fprintf(out, "Type the regatta id (%s) to confirm, anything else cancels: ", o.regattaID)
	line, _ := bufio.NewReader(in).ReadString('\n')
	if strings.TrimSpace(line) != o.regattaID {
		fmt.Fprintln(out, "Confirmation did not match - nothing was written.")
		return false
	}
	return true
}

func sortedByRaceLane(matches []laneMatch) []laneMatch {
	out := append([]laneMatch(nil), matches...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].RaceNumber != out[j].RaceNumber {
			return out[i].RaceNumber < out[j].RaceNumber
		}
		return out[i].Lane < out[j].Lane
	})
	return out
}

// printRequest prints one call's method, URL, and JSON body exactly as it
// would be marshaled and sent - used only for --dry-run output, never for
// an actual request.
func printRequest(out io.Writer, method, url string, body any) {
	fmt.Fprintf(out, "%s %s\n", method, url)
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		fmt.Fprintf(out, "(failed to marshal body: %v)\n\n", err)
		return
	}
	out.Write(b)
	fmt.Fprintln(out)
	fmt.Fprintln(out)
}

// printScheduleDryRun prints every call publish-schedule --confirm would
// make: CreateRaces/AssignLanes for any race not already in raceIDs, then
// the final /upload PUT - in that order, with real bodies and URLs, but
// without ever touching the network. A race not yet created can't have a
// real RaceId yet, so its lane-assignment URL and its RaceRecord in the
// /upload body both show a placeholder instead.
func printScheduleDryRun(out io.Writer, o publishOptions, baseURL string, publishable []laneMatch, raceIDs map[raceKey]int) {
	fmt.Fprintln(out, strings.Repeat("=", 60))
	fmt.Fprintln(out, "DRY RUN: publish-schedule - nothing below is sent.")
	fmt.Fprintln(out, strings.Repeat("=", 60))
	fmt.Fprintln(out)

	lanes := lanesByRaceKey(publishable)
	preview := make(map[raceKey]int, len(raceIDs))
	maps.Copy(preview, raceIDs)

	newByEvent := map[int][]int{}
	seen := map[raceKey]bool{}
	for k := range lanes {
		if seen[k] {
			continue
		}
		seen[k] = true
		if _, ok := raceIDs[k]; ok {
			continue
		}
		newByEvent[k.eventID] = append(newByEvent[k.eventID], k.raceNumber)
	}

	if len(newByEvent) == 0 {
		fmt.Fprintln(out, "Every race this run needs is already in race-ids.json - no CreateRaces/AssignLanes calls needed.")
		fmt.Fprintln(out)
	}

	var eventIDs []int
	for eventID := range newByEvent {
		eventIDs = append(eventIDs, eventID)
	}
	sort.Ints(eventIDs)

	for _, eventID := range eventIDs {
		raceNumbers := newByEvent[eventID]
		sort.Ints(raceNumbers)

		req := make([]regattacentral.RaceRecord, len(raceNumbers))
		for i, rn := range raceNumbers {
			req[i] = regattacentral.RaceRecord{UUID: newPlaceholderUUID(), DisplayNumber: strconv.Itoa(rn)}
		}
		printRequest(out, "POST", fmt.Sprintf("%sregattas/%s/events/%d/races", baseURL, o.regattaID, eventID), req)

		for _, rn := range raceNumbers {
			placeholder := fmt.Sprintf("<real-race-id-for-xlsm-race-%d>", rn)
			printRequest(out, "POST", fmt.Sprintf("%sregattas/%s/races/%s/lanes", baseURL, o.regattaID, placeholder), lanes[raceKey{eventID, rn}])
			preview[raceKey{eventID, rn}] = 0
		}
	}

	printRequest(out, "PUT", fmt.Sprintf("%sregattas/%s/upload", baseURL, o.regattaID), buildScheduleRequest(publishable, preview))
	if len(newByEvent) > 0 {
		fmt.Fprintln(out, "Note: raceId 0 in the /upload body above marks a race not yet created - CreateRaces's real assigned id would appear there instead once run for real.")
	}
}

// printResultsDryRun prints the /upload PUT publish-results --confirm would
// make, without touching the network. A race missing from raceIDs (meaning
// publish-schedule hasn't created it yet) shows raceId 0 with a note, rather
// than the hard error a real run would return.
func printResultsDryRun(out io.Writer, o publishOptions, baseURL string, publishable []laneMatch, raceIDs map[raceKey]int) {
	fmt.Fprintln(out, strings.Repeat("=", 60))
	fmt.Fprintln(out, "DRY RUN: publish-results - nothing below is sent.")
	fmt.Fprintln(out, strings.Repeat("=", 60))
	fmt.Fprintln(out)

	if missing := missingRaceIDs(publishable, raceIDs); len(missing) > 0 {
		fmt.Fprintf(out, "Note: %d race(s) are not yet in race-ids.json (run publish-schedule first) - shown below with a placeholder raceId of 0:\n", len(missing))
		for _, k := range missing {
			fmt.Fprintf(out, "  Race %d (RC event %d)\n", k.raceNumber, k.eventID)
		}
		fmt.Fprintln(out)
	}

	printRequest(out, "PUT", fmt.Sprintf("%sregattas/%s/upload", baseURL, o.regattaID), buildResultsRequest(publishable, raceIDs))
}

func runPublishSchedule(argv []string) error {
	o, err := parsePublishFlags("publish-schedule", argv)
	if err != nil {
		return err
	}
	rd, publishable, guessed, excluded, err := resolvedLanes(o)
	if err != nil {
		return err
	}

	if o.dryRun {
		summaryOpts := o
		summaryOpts.confirm = false
		printSummaryAndConfirm(os.Stdout, strings.NewReader(""), "schedule (lane draws only, no results)", summaryOpts, rd, publishable, guessed, excluded)
		raceIDs, err := loadRaceIDs(o.rcDir)
		if err != nil {
			return err
		}
		baseURL, err := resolveBaseURL(o)
		if err != nil {
			return err
		}
		printScheduleDryRun(os.Stdout, o, baseURL, publishable, raceIDs)
		return nil
	}

	if !printSummaryAndConfirm(os.Stdout, os.Stdin, "schedule (lane draws only, no results)", o, rd, publishable, guessed, excluded) {
		return nil
	}

	client, err := publishClient(o)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Races must be created on RegattaCentral - via the dedicated
	// CreateRaces/AssignLanes endpoints, not Upload - before anything can
	// reference them. raceIDs starts from any earlier publish-schedule run
	// so a re-run never creates duplicate races (see createRaces).
	raceIDs, loadErr := loadRaceIDs(o.rcDir)
	if loadErr != nil {
		return loadErr
	}
	raceIDs, createErr := createRaces(ctx, client, o.regattaID, publishable, raceIDs)
	// Persist whatever succeeded even on a partial failure, so a retry
	// doesn't re-create races that already exist.
	if saveErr := saveRaceIDs(o.rcDir, raceIDs); saveErr != nil {
		return fmt.Errorf("save %s after creating races: %w", raceIDsFileName, saveErr)
	}
	if createErr != nil {
		return fmt.Errorf("publish schedule: %w", createErr)
	}

	req := buildScheduleRequest(publishable, raceIDs)
	if err := client.Upload(ctx, o.regattaID, req); err != nil {
		return fmt.Errorf("publish schedule: %w", err)
	}
	fmt.Println("Schedule published. Verify it on RegattaCentral's own site before running publish-results.")
	return nil
}

func runPublishResults(argv []string) error {
	o, err := parsePublishFlags("publish-results", argv)
	if err != nil {
		return err
	}
	rd, publishable, guessed, excluded, err := resolvedLanes(o)
	if err != nil {
		return err
	}

	raceIDs, err := loadRaceIDs(o.rcDir)
	if err != nil {
		return err
	}

	if o.dryRun {
		summaryOpts := o
		summaryOpts.confirm = false
		printSummaryAndConfirm(os.Stdout, strings.NewReader(""), "results", summaryOpts, rd, publishable, guessed, excluded)
		baseURL, err := resolveBaseURL(o)
		if err != nil {
			return err
		}
		printResultsDryRun(os.Stdout, o, baseURL, publishable, raceIDs)
		return nil
	}

	if missing := missingRaceIDs(publishable, raceIDs); len(missing) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "publish results: %d race(s) have not been created on RegattaCentral yet - run publish-schedule first:\n", len(missing))
		for _, k := range missing {
			fmt.Fprintf(&b, "  Race %d (RC event %d)\n", k.raceNumber, k.eventID)
		}
		return errors.New(strings.TrimSuffix(b.String(), "\n"))
	}

	if !printSummaryAndConfirm(os.Stdout, os.Stdin, "results", o, rd, publishable, guessed, excluded) {
		return nil
	}

	client, err := publishClient(o)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	req := buildResultsRequest(publishable, raceIDs)
	if err := client.Upload(ctx, o.regattaID, req); err != nil {
		return fmt.Errorf("publish results: %w", err)
	}
	fmt.Println("Results published.")
	return nil
}
