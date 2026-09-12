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
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
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
	confirm                                                     bool
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
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rcreconcile %s --xlsm PATH --rc-dir DIR --regatta ID [--config PATH] [--secrets-file PATH] [--origin URL] [--confirm]\n\nflags:\n", name)
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

	baseURL := ""
	if o.configFile != "" {
		cfg, err := personacfg.Load(o.configFile)
		if err != nil {
			return nil, err
		}
		if rc, ok := cfg.RegattaCentralConfig(); ok {
			baseURL = rc.BaseURL
		}
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
// (statusMatched/statusGuessed with a numeric EntryID - the only lanes a
// live push ever sends), guessed (the subset of publishable that only
// resolved via --guess-ties, called out separately so the operator sees
// exactly which lanes are a best guess, not a confident match), and excluded
// (no resolvable entry, still ambiguous even after --guess-ties, or a
// non-numeric id this tool won't invent a placeholder UUID for on a real
// write - see the package doc comment).
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

// buildScheduleRequest builds the publish-schedule payload: one LaneRecord
// per publishable lane and a StatusDraw RaceRecord for every race that has
// one. SCR/EXH (via laneStatus) are lane-level facts already fully known
// from the historical xlsm, so including them at this stage - even though
// it's nominally "schedule, not results" - is correct.
func buildScheduleRequest(publishable []laneMatch) *regattacentral.UploadRequest {
	req := &regattacentral.UploadRequest{}
	races := map[int]bool{}
	for _, m := range publishable {
		id, _ := strconv.Atoi(m.Candidates[0].ID) // classifyForPublish already guaranteed this parses
		req.AddLane(regattacentral.LaneRecord{
			RaceNumber:    m.RaceNumber,
			Lane:          m.Lane,
			DisplayNumber: extractBoatLabel(m.AdditionalInfo),
			EntryID:       id,
			Status:        laneStatus(m.Place, m.LaneClass),
		})
		races[m.RaceNumber] = true
	}
	for raceNumber := range races {
		req.SetRaceStatus(raceNumber, regattacentral.StatusDraw)
	}
	return req
}

// buildResultsRequest builds the publish-results payload: one ResultRecord
// per publishable lane with a parseable finish time, and a StatusOfficial
// RaceRecord for every race that gets one. No LaneRecords - those were
// already sent by an earlier publish-schedule run (assumeLanesUploaded:
// true, see runPublishResults).
func buildResultsRequest(publishable []laneMatch) *regattacentral.UploadRequest {
	req := &regattacentral.UploadRequest{}
	races := map[int]bool{}
	for _, m := range publishable {
		if d, ok := parseRaceTime(m.Time); ok {
			req.AddFinish(m.RaceNumber, m.Lane, d)
			races[m.RaceNumber] = true
		}
	}
	for raceNumber := range races {
		req.SetRaceStatus(raceNumber, regattacentral.StatusOfficial)
	}
	return req
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

func runPublishSchedule(argv []string) error {
	o, err := parsePublishFlags("publish-schedule", argv)
	if err != nil {
		return err
	}
	rd, publishable, guessed, excluded, err := resolvedLanes(o)
	if err != nil {
		return err
	}
	req := buildScheduleRequest(publishable)

	if !printSummaryAndConfirm(os.Stdout, os.Stdin, "schedule (lane draws only, no results)", o, rd, publishable, guessed, excluded) {
		return nil
	}

	client, err := publishClient(o)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := client.Upload(ctx, o.regattaID, req, false); err != nil {
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
	req := buildResultsRequest(publishable)

	if !printSummaryAndConfirm(os.Stdout, os.Stdin, "results", o, rd, publishable, guessed, excluded) {
		return nil
	}

	client, err := publishClient(o)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := client.Upload(ctx, o.regattaID, req, true); err != nil {
		return fmt.Errorf("publish results: %w", err)
	}
	fmt.Println("Results published.")
	return nil
}
