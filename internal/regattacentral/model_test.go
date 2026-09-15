package regattacentral

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAddLaneNestsUnderEventAndRace(t *testing.T) {
	var u UploadRequest
	u.AddLane(100, 5, LaneRecord{Lane: 1, EntryID: 4821})

	if len(u.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(u.Events))
	}
	ev := u.Events[0]
	if ev.EventID != 100 {
		t.Errorf("EventID = %d, want 100", ev.EventID)
	}
	if len(ev.Races) != 1 {
		t.Fatalf("races = %d, want 1", len(ev.Races))
	}
	race := ev.Races[0]
	if race.RaceID != 5 || race.EventID != 100 {
		t.Errorf("race = %+v, want RaceID 5 EventID 100", race)
	}
	if len(race.Lanes) != 1 || race.Lanes[0].EntryID != 4821 {
		t.Errorf("lanes = %+v, want one lane with EntryID 4821", race.Lanes)
	}
}

func TestAddLaneAppendsSecondLaneToSameRace(t *testing.T) {
	var u UploadRequest
	u.AddLane(100, 5, LaneRecord{Lane: 1})
	u.AddLane(100, 5, LaneRecord{Lane: 2})

	if len(u.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(u.Events))
	}
	if len(u.Events[0].Races) != 1 {
		t.Fatalf("races = %d, want 1", len(u.Events[0].Races))
	}
	if len(u.Events[0].Races[0].Lanes) != 2 {
		t.Fatalf("lanes = %d, want 2", len(u.Events[0].Races[0].Lanes))
	}
}

func TestAddLaneSplitsAcrossEventsSharingRaceID(t *testing.T) {
	var u UploadRequest
	u.AddLane(100, 9, LaneRecord{Lane: 1})
	u.AddLane(200, 9, LaneRecord{Lane: 2})

	if len(u.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(u.Events))
	}
	for _, ev := range u.Events {
		if len(ev.Races) != 1 {
			t.Fatalf("event %d races = %d, want 1", ev.EventID, len(ev.Races))
		}
		race := ev.Races[0]
		if race.RaceID != 9 {
			t.Errorf("event %d race RaceID = %d, want 9", ev.EventID, race.RaceID)
		}
		if race.EventID != ev.EventID {
			t.Errorf("event %d race EventID = %d, want %d", ev.EventID, race.EventID, ev.EventID)
		}
		if len(race.Lanes) != 1 {
			t.Errorf("event %d race lanes = %d, want 1", ev.EventID, len(race.Lanes))
		}
	}
}

func TestSetRaceStatusUpdatesInPlacePerEvent(t *testing.T) {
	var u UploadRequest
	u.SetRaceStatus(100, 3, "3", StatusPreDraw)
	u.SetRaceStatus(100, 4, "4", StatusDraw)
	u.SetRaceStatus(100, 3, "3", StatusRacing) // update, not append

	if len(u.Events) != 1 || len(u.Events[0].Races) != 2 {
		t.Fatalf("events/races = %+v, want 1 event with 2 races", u.Events)
	}
	race3 := u.Events[0].Races[0]
	if race3.RaceID != 3 || race3.Status != StatusRacing || race3.DisplayNumber != "3" {
		t.Errorf("race 3 = %+v, want status Racing, displayNumber 3", race3)
	}

	// Same RaceID under a different EventID is a distinct, independently
	// updatable RaceRecord - the intentional mixed-boat-class split.
	u.SetRaceStatus(200, 3, "3", StatusOfficial)
	if len(u.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(u.Events))
	}
	if u.Events[0].Races[0].Status != StatusRacing {
		t.Errorf("event 100 race 3 status changed unexpectedly: %+v", u.Events[0].Races[0])
	}
	if u.Events[1].Races[0].RaceID != 3 || u.Events[1].Races[0].Status != StatusOfficial {
		t.Errorf("event 200 race 3 = %+v, want RaceID 3 status Official", u.Events[1].Races[0])
	}
}

func TestAddFinishNestsResultUnderExistingLane(t *testing.T) {
	var u UploadRequest
	u.AddLane(100, 7, LaneRecord{Lane: 4, EntryID: 555})
	u.AddFinish(100, 7, 4, 6*time.Minute+12*time.Second+500*time.Millisecond)

	lanes := u.Events[0].Races[0].Lanes
	if len(lanes) != 1 {
		t.Fatalf("lanes = %d, want 1 (result must land in the existing lane, not a sibling)", len(lanes))
	}
	if lanes[0].EntryID != 555 {
		t.Errorf("EntryID = %d, want 555 (existing lane data lost)", lanes[0].EntryID)
	}
	if len(lanes[0].Results) != 1 {
		t.Fatalf("results = %d, want 1", len(lanes[0].Results))
	}
	r := lanes[0].Results[0]
	if r.TimingMilestoneID != MilestoneFinish {
		t.Errorf("milestone = %d, want %d", r.TimingMilestoneID, MilestoneFinish)
	}
	if r.Time != 372500 {
		t.Errorf("time = %d ms, want 372500", r.Time)
	}
}

func TestAddFinishAutoVivifiesLaneWhenCalledFirst(t *testing.T) {
	var u UploadRequest
	u.AddFinish(100, 7, 4, time.Minute)

	lanes := u.Events[0].Races[0].Lanes
	if len(lanes) != 1 || lanes[0].Lane != 4 {
		t.Fatalf("lanes = %+v, want one minimal lane 4", lanes)
	}
	if len(lanes[0].Results) != 1 {
		t.Fatalf("results = %d, want 1", len(lanes[0].Results))
	}
}

func TestValidateRejectsZeroEventID(t *testing.T) {
	req := UploadRequest{Events: []EventRecord{{EventID: 0, Races: []RaceRecord{{RaceID: 1}}}}}
	if err := req.Validate(); err == nil {
		t.Error("Validate() = nil, want error for eventId 0")
	}
}

func TestValidateRejectsZeroRaceID(t *testing.T) {
	req := UploadRequest{Events: []EventRecord{{EventID: 100, Races: []RaceRecord{{RaceID: 0}}}}}
	if err := req.Validate(); err == nil {
		t.Error("Validate() = nil, want error for raceId 0")
	}
}

func TestValidateRejectsDuplicateRaceIDWithinOneEvent(t *testing.T) {
	req := UploadRequest{Events: []EventRecord{{EventID: 100, Races: []RaceRecord{
		{RaceID: 5}, {RaceID: 5},
	}}}}
	if err := req.Validate(); err == nil {
		t.Error("Validate() = nil, want error for duplicate raceId within one event")
	}
}

func TestValidateRejectsDuplicateLaneWithinOneRace(t *testing.T) {
	req := UploadRequest{Events: []EventRecord{{EventID: 100, Races: []RaceRecord{
		{RaceID: 5, Lanes: []LaneRecord{{Lane: 1}, {Lane: 1}}},
	}}}}
	if err := req.Validate(); err == nil {
		t.Error("Validate() = nil, want error for duplicate lane within one race")
	}
}

func TestValidateAllowsSameRaceIDAcrossTwoEvents(t *testing.T) {
	req := UploadRequest{Events: []EventRecord{
		{EventID: 100, Races: []RaceRecord{{RaceID: 5, EventID: 100}}},
		{EventID: 200, Races: []RaceRecord{{RaceID: 5, EventID: 200}}},
	}}
	if err := req.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil (same raceId across events is the intentional split)", err)
	}
}

func TestUploadRequestMarshalsNestedShape(t *testing.T) {
	var req UploadRequest
	req.AddLane(100, 5, LaneRecord{Lane: 1, EntryID: 4821, Status: LaneOK})
	req.SetRaceStatus(100, 5, "5", StatusDraw)
	req.AddLane(200, 6, LaneRecord{Lane: 1, EntryID: 9001})
	req.SetRaceStatus(200, 6, "6", StatusOfficial)
	req.AddFinish(200, 6, 1, time.Minute)

	b, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	events, ok := out["events"].([]any)
	if !ok || len(events) != 2 {
		t.Fatalf("events = %v, want a 2-element array", out["events"])
	}
	ev0 := events[0].(map[string]any)
	if ev0["eventId"] != float64(100) {
		t.Errorf("events[0].eventId = %v, want 100", ev0["eventId"])
	}
	races0 := ev0["races"].([]any)
	race0 := races0[0].(map[string]any)
	if race0["raceId"] != float64(5) || race0["status"] != "Draw" {
		t.Errorf("events[0].races[0] = %v, want raceId 5 status Draw", race0)
	}
	if _, hasUUID := race0["uuid"]; hasUUID {
		t.Errorf("events[0].races[0] has uuid, want omitted (empty, omitempty)")
	}
	lanes0 := race0["lanes"].([]any)
	if _, hasResults := lanes0[0].(map[string]any)["results"]; hasResults {
		t.Errorf("events[0].races[0].lanes[0] has results, want omitted (no finish sent)")
	}

	ev1 := events[1].(map[string]any)
	race1 := ev1["races"].([]any)[0].(map[string]any)
	lane1 := race1["lanes"].([]any)[0].(map[string]any)
	results1, ok := lane1["results"].([]any)
	if !ok || len(results1) != 1 {
		t.Fatalf("events[1].races[0].lanes[0].results = %v, want one result", lane1["results"])
	}
	result1 := results1[0].(map[string]any)
	if result1["timingMilestoneId"] != float64(MilestoneFinish) || result1["time"] != float64(60000) {
		t.Errorf("result = %v, want timingMilestoneId %d time 60000", result1, MilestoneFinish)
	}
}
