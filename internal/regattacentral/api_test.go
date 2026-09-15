package regattacentral

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestCreateRacesPostsAndParsesResponse(t *testing.T) {
	var (
		method, path string
		body         []RaceRecord
	)
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":true,"count":1,"data":[{"raceId":88123,"displayNumber":"9"}],"links":null,"messages":null}`)
	})
	c := ts.client(t, Config{})

	races, err := c.CreateRaces(context.Background(), "R7", "1001", []RaceRecord{{DisplayNumber: "9"}})
	if err != nil {
		t.Fatalf("CreateRaces: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %s, want POST", method)
	}
	// "regattas" (plural) - RC's own docs showed singular "regatta" here,
	// but that was a documentation bug: a live POST against the singular
	// path 404'd, while plural reached the real controller (confirmed
	// empirically against the live API - see CreateRaces's doc comment).
	if path != "/v4.0/regattas/R7/events/1001/races" {
		t.Fatalf("path = %q, want /v4.0/regattas/R7/events/1001/races (plural \"regattas\")", path)
	}
	if len(body) != 1 || body[0].DisplayNumber != "9" {
		t.Fatalf("request body = %+v, want one race with displayNumber 9", body)
	}
	if len(races) != 1 || races[0].RaceID != 88123 || races[0].DisplayNumber != "9" {
		t.Fatalf("races = %+v, want one race with RaceID 88123", races)
	}
}

func TestCreateRacesErrorsOnSuccessFalse(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":false,"count":0,"data":null,"links":null,"messages":["Failed"]}`)
	})
	c := ts.client(t, Config{})

	if _, err := c.CreateRaces(context.Background(), "R7", "1001", []RaceRecord{{DisplayNumber: "9"}}); err == nil {
		t.Error("CreateRaces() = nil error, want one for a success:false response")
	}
}

func TestAssignLanesPostsToRealRaceID(t *testing.T) {
	var (
		method, path string
		body         []LaneRecord
	)
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":true,"count":1,"data":[{"lane":3,"entryId":555}],"links":null,"messages":null}`)
	})
	c := ts.client(t, Config{})

	lanes, err := c.AssignLanes(context.Background(), "R7", 88123, []LaneRecord{{Lane: 3, EntryID: 555}})
	if err != nil {
		t.Fatalf("AssignLanes: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %s, want POST", method)
	}
	if path != "/v4.0/regattas/R7/races/88123/lanes" {
		t.Fatalf("path = %q, want /v4.0/regattas/R7/races/88123/lanes (plural \"regattas\")", path)
	}
	if len(body) != 1 || body[0].EntryID != 555 {
		t.Fatalf("request body = %+v, want one lane with EntryID 555", body)
	}
	if len(lanes) != 1 || lanes[0].Lane != 3 || lanes[0].EntryID != 555 {
		t.Fatalf("lanes = %+v, want one lane matching the request", lanes)
	}
}
