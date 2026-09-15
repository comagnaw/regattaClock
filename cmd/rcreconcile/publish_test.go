package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

func TestClassifyForPublish(t *testing.T) {
	matches := []laneMatch{
		{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", Status: statusMatched, Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
		{RaceNumber: 1, Lane: 2, SchoolName: "Justice High", Status: statusGuessed, Candidates: []rcEntry{{ID: "65", EventID: "100"}, {ID: "64", EventID: "100"}}},
		{RaceNumber: 1, Lane: 3, SchoolName: "North Haverbrook", Status: statusUnmatched},
		{RaceNumber: 1, Lane: 4, SchoolName: "Ambiguous Twins", Status: statusAmbiguous, Candidates: []rcEntry{{ID: "1", EventID: "100"}, {ID: "2", EventID: "100"}}},
		{RaceNumber: 1, Lane: 5, SchoolName: "Non Numeric", Status: statusMatched, Candidates: []rcEntry{{ID: "not-a-number", EventID: "100"}}},
		{RaceNumber: 1, Lane: 6, SchoolName: "Non Numeric Event", Status: statusMatched, Candidates: []rcEntry{{ID: "9999", EventID: "not-a-number"}}},
	}

	publishable, guessed, excluded := classifyForPublish(matches)

	if len(publishable) != 2 {
		t.Fatalf("publishable = %d, want 2 (lanes 1 and 2): %+v", len(publishable), publishable)
	}
	if publishable[0].Lane != 1 || publishable[1].Lane != 2 {
		t.Errorf("publishable lanes = [%d, %d], want [1, 2]", publishable[0].Lane, publishable[1].Lane)
	}
	if len(guessed) != 1 || guessed[0].Lane != 2 {
		t.Errorf("guessed = %+v, want just lane 2", guessed)
	}
	if len(excluded) != 4 {
		t.Fatalf("excluded = %d, want 4 (unmatched, ambiguous, non-numeric id, non-numeric eventId): %+v", len(excluded), excluded)
	}
	excludedLanes := map[int]bool{}
	for _, m := range excluded {
		excludedLanes[m.Lane] = true
	}
	if !excludedLanes[3] || !excludedLanes[4] || !excludedLanes[5] || !excludedLanes[6] {
		t.Errorf("excluded lanes = %v, want {3, 4, 5, 6}", excludedLanes)
	}
}

func TestBuildScheduleRequestNestsUnderEventAndRace(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 1, Lane: 1, AdditionalInfo: "A", Place: "1", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
		{RaceNumber: 1, Lane: 2, Place: "SCR", Candidates: []rcEntry{{ID: "4822", EventID: "100"}}},
		{RaceNumber: 2, Lane: 1, Candidates: []rcEntry{{ID: "9001", EventID: "200"}}},
	}
	raceIDs := map[raceKey]int{
		{eventID: 100, raceNumber: 1}: 55001,
		{eventID: 200, raceNumber: 2}: 55002,
	}

	req := buildScheduleRequest(publishable, raceIDs)

	if len(req.Events) != 2 {
		t.Fatalf("events = %d, want 2 (one per distinct RC event): %+v", len(req.Events), req.Events)
	}
	var race1, race2 *regattacentral.RaceRecord
	for i := range req.Events {
		for j := range req.Events[i].Races {
			r := &req.Events[i].Races[j]
			switch r.RaceID {
			case 55001:
				race1 = r
			case 55002:
				race2 = r
			}
		}
	}
	if race1 == nil || race2 == nil {
		t.Fatalf("expected races with the real ids from raceIDs, got %+v", req.Events)
	}
	if len(race1.Lanes) != 2 {
		t.Fatalf("race 1 lanes = %d, want 2: %+v", len(race1.Lanes), race1.Lanes)
	}
	if race1.Lanes[0].EntryID != 4821 || race1.Lanes[0].DisplayNumber != "A" {
		t.Errorf("race 1 lane 0 = %+v, want EntryID 4821 and DisplayNumber \"A\"", race1.Lanes[0])
	}
	if race1.Lanes[1].Status != regattacentral.LaneScratched {
		t.Errorf("race 1 lane 1 status = %q, want LaneScratched from Place=SCR", race1.Lanes[1].Status)
	}
	if race1.Status != regattacentral.StatusDraw || race2.Status != regattacentral.StatusDraw {
		t.Errorf("race1=%+v race2=%+v, want both StatusDraw", race1, race2)
	}
	if race1.DisplayNumber != "1" || race2.DisplayNumber != "2" {
		t.Errorf("displayNumbers = %q/%q, want \"1\"/\"2\" (the xlsm race number, even though RaceID is now the real id)", race1.DisplayNumber, race2.DisplayNumber)
	}
}

func TestBuildScheduleRequestSplitsRaceAcrossTwoEvents(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 5, Lane: 1, SchoolName: "Home Class", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
		{RaceNumber: 5, Lane: 2, SchoolName: "Combined Exhibition Class", Candidates: []rcEntry{{ID: "9001", EventID: "200"}}},
	}
	raceIDs := map[raceKey]int{
		{eventID: 100, raceNumber: 5}: 66001,
		{eventID: 200, raceNumber: 5}: 66002,
	}

	req := buildScheduleRequest(publishable, raceIDs)

	if len(req.Events) != 2 {
		t.Fatalf("events = %d, want 2 (race 5 split across two events): %+v", len(req.Events), req.Events)
	}
	seenRaceIDs := map[int]bool{}
	for _, ev := range req.Events {
		if len(ev.Races) != 1 {
			t.Fatalf("event %d races = %d, want 1", ev.EventID, len(ev.Races))
		}
		race := ev.Races[0]
		wantID := raceIDs[raceKey{eventID: ev.EventID, raceNumber: 5}]
		if race.RaceID != wantID {
			t.Errorf("event %d race RaceID = %d, want %d (its own real id, not shared with the other event's portion)", ev.EventID, race.RaceID, wantID)
		}
		seenRaceIDs[race.RaceID] = true
		if race.EventID != ev.EventID {
			t.Errorf("event %d race EventID = %d, want %d", ev.EventID, race.EventID, ev.EventID)
		}
		if race.DisplayNumber != "5" {
			t.Errorf("event %d race DisplayNumber = %q, want \"5\" (shared human label across the split)", ev.EventID, race.DisplayNumber)
		}
		if len(race.Lanes) != 1 {
			t.Errorf("event %d race lanes = %d, want 1 (only its own event's lane)", ev.EventID, len(race.Lanes))
		}
	}
	if len(seenRaceIDs) != 2 {
		t.Errorf("distinct RaceIDs across the split = %d, want 2 (66001 and 66002 must both appear)", len(seenRaceIDs))
	}
}

func TestBuildResultsRequestIncludesFullLaneAlongsideResult(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 1, Lane: 1, Place: "1", Time: "6:12.5", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
	}
	raceIDs := map[raceKey]int{{eventID: 100, raceNumber: 1}: 77001}

	req := buildResultsRequest(publishable, raceIDs)

	if len(req.Events) != 1 || len(req.Events[0].Races) != 1 {
		t.Fatalf("events = %+v, want one event with one race", req.Events)
	}
	race := req.Events[0].Races[0]
	if race.RaceID != 77001 {
		t.Errorf("race RaceID = %d, want 77001 (the real id from raceIDs)", race.RaceID)
	}
	if race.Status != regattacentral.StatusOfficial {
		t.Errorf("race status = %q, want StatusOfficial", race.Status)
	}
	if len(race.Lanes) != 1 {
		t.Fatalf("lanes = %d, want 1 - the full lane must be resent alongside the result", len(race.Lanes))
	}
	lane := race.Lanes[0]
	if lane.EntryID != 4821 {
		t.Errorf("lane EntryID = %d, want 4821 (full lane data, not just the result)", lane.EntryID)
	}
	if len(lane.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(lane.Results))
	}
	want := (6*time.Minute + 12*time.Second + 500*time.Millisecond).Milliseconds()
	if lane.Results[0].Time != want {
		t.Errorf("result time = %d, want %d", lane.Results[0].Time, want)
	}
}

func TestBuildResultsRequestSkipsLanesWithNoParseableTime(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 1, Lane: 1, Time: "6:12.5", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
		{RaceNumber: 1, Lane: 2, Time: "", Candidates: []rcEntry{{ID: "4822", EventID: "100"}}}, // no time yet
	}
	raceIDs := map[raceKey]int{{eventID: 100, raceNumber: 1}: 77002}

	req := buildResultsRequest(publishable, raceIDs)

	if len(req.Events) != 1 || len(req.Events[0].Races) != 1 {
		t.Fatalf("events = %+v, want one event with one race", req.Events)
	}
	lanes := req.Events[0].Races[0].Lanes
	if len(lanes) != 1 || lanes[0].Lane != 1 {
		t.Fatalf("lanes = %+v, want exactly lane 1 (lane 2 has no parseable time)", lanes)
	}
}

func TestPrintSummaryAndConfirmWithoutConfirmNeverPrompts(t *testing.T) {
	rd := &reader.RegattaData{Races: []reader.RaceData{raceWithLanes(1, "Varsity 8", map[int]reader.RaceEntry{
		1: {SchoolName: "Springfield HS"},
	})}}
	publishable := []laneMatch{{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}}}

	var out strings.Builder
	// A reader that errors on any read - proves the non-confirm path never
	// touches stdin at all.
	proceed := printSummaryAndConfirm(&out, failingReader{}, "schedule", publishOptions{regattaID: "999", confirm: false}, rd, publishable, nil, nil)

	if proceed {
		t.Error("proceed = true, want false when --confirm is not set")
	}
	if !strings.Contains(out.String(), "--confirm not set") {
		t.Errorf("summary = %q, want it to say --confirm was not set", out.String())
	}
}

func TestPrintSummaryAndConfirmRequiresExactRegattaID(t *testing.T) {
	rd := &reader.RegattaData{Races: []reader.RaceData{raceWithLanes(1, "Varsity 8", map[int]reader.RaceEntry{
		1: {SchoolName: "Springfield HS"},
	})}}
	publishable := []laneMatch{{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}}}
	guessed := []laneMatch{publishable[0]}
	excluded := []laneMatch{{RaceNumber: 2, Lane: 1, SchoolName: "North Haverbrook"}}

	var out strings.Builder
	proceed := printSummaryAndConfirm(&out, strings.NewReader("wrong-id\n"), "schedule", publishOptions{regattaID: "999", confirm: true}, rd, publishable, guessed, excluded)
	if proceed {
		t.Error("proceed = true, want false for a mismatched confirmation")
	}
	if !strings.Contains(out.String(), "North Haverbrook") {
		t.Errorf("summary = %q, want the excluded lane listed by school name", out.String())
	}
	if !strings.Contains(out.String(), "RC entry 4821") {
		t.Errorf("summary = %q, want the guessed lane's picked entry id shown", out.String())
	}

	out.Reset()
	proceed = printSummaryAndConfirm(&out, strings.NewReader("999\n"), "schedule", publishOptions{regattaID: "999", confirm: true}, rd, publishable, guessed, excluded)
	if !proceed {
		t.Error("proceed = false, want true when the typed id matches exactly")
	}
}

func TestPrintSummaryAndConfirmFlagsSplitRace(t *testing.T) {
	rd := &reader.RegattaData{Races: []reader.RaceData{raceWithLanes(9, "Varsity 8", map[int]reader.RaceEntry{
		1: {SchoolName: "Home Class"},
		2: {SchoolName: "Combined Exhibition Class"},
	})}}
	publishable := []laneMatch{
		{RaceNumber: 9, Lane: 1, SchoolName: "Home Class", Candidates: []rcEntry{{ID: "4821", EventID: "1001"}}},
		{RaceNumber: 9, Lane: 2, SchoolName: "Combined Exhibition Class", Candidates: []rcEntry{{ID: "9001", EventID: "1042"}}},
	}

	var out strings.Builder
	printSummaryAndConfirm(&out, strings.NewReader(""), "schedule", publishOptions{regattaID: "999", confirm: false}, rd, publishable, nil, nil)

	if !strings.Contains(out.String(), "SPLIT ACROSS MULTIPLE RC EVENTS") {
		t.Errorf("summary = %q, want a split-race warning", out.String())
	}
	if !strings.Contains(out.String(), "Race 9: RC events 1001, 1042") {
		t.Errorf("summary = %q, want the race number and both event ids named", out.String())
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	panic("stdin must not be read when --confirm is not set")
}

func TestLoadRaceIDsMissingFileReturnsEmpty(t *testing.T) {
	ids, err := loadRaceIDs(t.TempDir())
	if err != nil {
		t.Fatalf("loadRaceIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("ids = %v, want empty for a missing file", ids)
	}
}

func TestSaveRaceIDsRoundTripsThroughLoadRaceIDs(t *testing.T) {
	dir := t.TempDir()
	want := map[raceKey]int{
		{eventID: 100, raceNumber: 1}: 88001,
		{eventID: 100, raceNumber: 2}: 88002,
		{eventID: 200, raceNumber: 1}: 88003,
	}
	if err := saveRaceIDs(dir, want); err != nil {
		t.Fatalf("saveRaceIDs: %v", err)
	}
	got, err := loadRaceIDs(dir)
	if err != nil {
		t.Fatalf("loadRaceIDs: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("ids[%+v] = %d, want %d", k, got[k], v)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, raceIDsFileName)); err != nil {
		t.Errorf("expected %s to exist: %v", raceIDsFileName, err)
	}
}

func TestMissingRaceIDsListsUnresolvedRaces(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 1, Lane: 1, Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
		{RaceNumber: 2, Lane: 1, Candidates: []rcEntry{{ID: "4822", EventID: "100"}}},
	}
	raceIDs := map[raceKey]int{{eventID: 100, raceNumber: 1}: 88001}

	missing := missingRaceIDs(publishable, raceIDs)
	if len(missing) != 1 || missing[0] != (raceKey{eventID: 100, raceNumber: 2}) {
		t.Fatalf("missing = %+v, want just {100, 2}", missing)
	}

	// Fully resolved: nothing missing.
	raceIDs[raceKey{eventID: 100, raceNumber: 2}] = 88002
	if missing := missingRaceIDs(publishable, raceIDs); len(missing) != 0 {
		t.Errorf("missing = %+v, want none once every race has a real id", missing)
	}
}

// testRCServer fakes just enough of RegattaCentral's real API for
// createRaces: an OAuth2 token endpoint, and the two dedicated creation
// endpoints (POST regattas/{id}/events/{eventId}/races and POST
// regattas/{id}/races/{raceId}/lanes). It records every races/lanes call it
// receives so tests can assert on exactly what was sent.
type testRCServer struct {
	*httptest.Server
	mu         sync.Mutex
	racesCalls []string                    // event ids CreateRaces was called for, in order
	racesSent  []regattacentral.RaceRecord // every race record CreateRaces sent, across all calls
	lanesCalls []int                       // race ids AssignLanes was called for, in order
	nextRaceID int
}

func newTestRCServer(t *testing.T) *testRCServer {
	t.Helper()
	s := &testRCServer{nextRaceID: 90000}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth2/api/token"):
			io.WriteString(w, `{"access_token":"tok","refresh_token":"r","expires_in":3600}`)
		case strings.Contains(r.URL.Path, "/races") && strings.HasSuffix(r.URL.Path, "/races"):
			var races []regattacentral.RaceRecord
			json.NewDecoder(r.Body).Decode(&races)
			s.mu.Lock()
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			eventID := parts[len(parts)-2] // .../events/{eventId}/races
			s.racesCalls = append(s.racesCalls, eventID)
			s.racesSent = append(s.racesSent, races...)
			for i := range races {
				races[i].RaceID = s.nextRaceID
				s.nextRaceID++
			}
			s.mu.Unlock()
			body, _ := json.Marshal(map[string]any{"success": true, "count": len(races), "data": races})
			w.Write(body)
		case strings.HasSuffix(r.URL.Path, "/lanes"):
			var lanes []regattacentral.LaneRecord
			json.NewDecoder(r.Body).Decode(&lanes)
			s.mu.Lock()
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			raceIDStr := parts[len(parts)-2] // .../races/{raceId}/lanes
			var raceID int
			fmt.Sscanf(raceIDStr, "%d", &raceID)
			s.lanesCalls = append(s.lanesCalls, raceID)
			s.mu.Unlock()
			body, _ := json.Marshal(map[string]any{"success": true, "count": len(lanes), "data": lanes})
			w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"success":false}`)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *testRCServer) client(t *testing.T) *regattacentral.Client {
	t.Helper()
	c, err := regattacentral.New(regattacentral.Config{
		Credentials: regattacentral.Credentials{ClientID: "c", ClientSecret: "s", Username: "u", Password: "p"},
		TokenURL:    s.URL + "/oauth2/api/token",
		BaseURL:     s.URL + "/v4.0/",
		HTTPClient:  s.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCreateRacesCreatesNewAndSkipsAlreadyKnown(t *testing.T) {
	srv := newTestRCServer(t)
	client := srv.client(t)

	publishable := []laneMatch{
		{RaceNumber: 1, Lane: 1, Candidates: []rcEntry{{ID: "4821", EventID: "100"}}}, // already created
		{RaceNumber: 2, Lane: 1, Candidates: []rcEntry{{ID: "4822", EventID: "100"}}}, // new
	}
	existing := map[raceKey]int{{eventID: 100, raceNumber: 1}: 12345}

	updated, err := createRaces(context.Background(), client, "R1", publishable, existing)
	if err != nil {
		t.Fatalf("createRaces: %v", err)
	}

	if updated[raceKey{eventID: 100, raceNumber: 1}] != 12345 {
		t.Errorf("race 1's id changed to %d, want it to stay 12345 (already created)", updated[raceKey{eventID: 100, raceNumber: 1}])
	}
	newID, ok := updated[raceKey{eventID: 100, raceNumber: 2}]
	if !ok || newID == 0 {
		t.Fatalf("race 2 has no real id after createRaces: %v", updated)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.racesCalls) != 1 || srv.racesCalls[0] != "100" {
		t.Errorf("CreateRaces calls = %v, want exactly one for event 100 (race 1 already existed, skip it)", srv.racesCalls)
	}
	if len(srv.lanesCalls) != 1 || srv.lanesCalls[0] != newID {
		t.Errorf("AssignLanes calls = %v, want exactly one for the newly created race %d", srv.lanesCalls, newID)
	}
	if len(srv.racesSent) != 1 || srv.racesSent[0].UUID == "" {
		t.Fatalf("races sent to CreateRaces = %+v, want exactly one with a non-empty UUID", srv.racesSent)
	}
	if srv.racesSent[0].RaceID != 0 {
		t.Errorf("race sent to CreateRaces has RaceID = %d, want 0 (a new entity)", srv.racesSent[0].RaceID)
	}
}

func TestResolveBaseURLDefaultsWhenNoConfig(t *testing.T) {
	got, err := resolveBaseURL(publishOptions{})
	if err != nil {
		t.Fatalf("resolveBaseURL: %v", err)
	}
	if got != regattacentral.DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", got, regattacentral.DefaultBaseURL)
	}
	if !strings.HasSuffix(got, "/") {
		t.Errorf("baseURL = %q, want a trailing slash", got)
	}
}

func TestPrintScheduleDryRunShowsCreateAssignAndUpload(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 9, Lane: 1, AdditionalInfo: "A", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
	}
	var out strings.Builder
	printScheduleDryRun(&out, publishOptions{regattaID: "R1"}, "https://api.example.com/v4.0/", publishable, map[raceKey]int{})

	body := out.String()
	if !strings.Contains(body, "POST https://api.example.com/v4.0/regattas/R1/events/100/races") {
		t.Errorf("output missing the CreateRaces URL: %s", body)
	}
	if !strings.Contains(body, "POST https://api.example.com/v4.0/regattas/R1/races/<real-race-id-for-xlsm-race-9>/lanes") {
		t.Errorf("output missing the AssignLanes URL with a placeholder race id: %s", body)
	}
	if !strings.Contains(body, "PUT https://api.example.com/v4.0/regattas/R1/upload") {
		t.Errorf("output missing the /upload URL: %s", body)
	}
	if !strings.Contains(body, `"displayNumber": "9"`) {
		t.Errorf("output missing the race's displayNumber in a request body: %s", body)
	}
	if !strings.Contains(body, `"entryId": 4821`) {
		t.Errorf("output missing the lane's entryId in a request body: %s", body)
	}
	if !strings.Contains(body, `"uuid":`) {
		t.Errorf("output missing a uuid field in the CreateRaces request body: %s", body)
	}
	if !strings.Contains(body, "raceId 0 in the /upload body above marks a race not yet created") {
		t.Errorf("output missing the placeholder-raceId note: %s", body)
	}
}

func TestPrintScheduleDryRunSkipsCreateWhenAlreadyKnown(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 9, Lane: 1, Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
	}
	raceIDs := map[raceKey]int{{eventID: 100, raceNumber: 9}: 88123}

	var out strings.Builder
	printScheduleDryRun(&out, publishOptions{regattaID: "R1"}, "https://api.example.com/v4.0/", publishable, raceIDs)

	body := out.String()
	if strings.Contains(body, "POST https://api.example.com/v4.0/regattas/R1/events") {
		t.Errorf("output should not show a CreateRaces call for an already-created race: %s", body)
	}
	if !strings.Contains(body, "already in race-ids.json") {
		t.Errorf("output missing the already-created note: %s", body)
	}
	if !strings.Contains(body, `"raceId": 88123`) {
		t.Errorf("output should show the real raceId 88123 in the /upload body: %s", body)
	}
}

func TestPrintResultsDryRunFlagsMissingRaces(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 9, Lane: 1, Time: "6:12.5", Candidates: []rcEntry{{ID: "4821", EventID: "100"}}},
	}

	var out strings.Builder
	printResultsDryRun(&out, publishOptions{regattaID: "R1"}, "https://api.example.com/v4.0/", publishable, map[raceKey]int{})

	body := out.String()
	if !strings.Contains(body, "Race 9 (RC event 100)") {
		t.Errorf("output missing the missing-race note: %s", body)
	}
	if !strings.Contains(body, "PUT https://api.example.com/v4.0/regattas/R1/upload") {
		t.Errorf("output missing the /upload URL: %s", body)
	}
	if !strings.Contains(body, `"raceId": 0`) {
		t.Errorf("output should show placeholder raceId 0 for the missing race: %s", body)
	}
}
