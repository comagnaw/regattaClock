package regattacentral

import (
	"fmt"
	"time"
)

// The read side of the model is intentionally thin: Bulk and the other GETs
// return raw JSON (json.RawMessage) for now. cmd/rcprobe captures real responses
// against a live staff account to inform typed structs in a follow-up; those
// captures contain athlete PII and are never committed, so the eventual test
// fixtures are hand-authored with synthetic data. The upload side below is
// typed, because the Cookbook's prose and its LaneConstructor example pin down
// the field and enum names - though those are still PROVISIONAL until a
// round-trip against the live API confirms them.

// RaceStatus is a RegattaCentral race status (Cookbook §13). Regularly advancing
// this as the regatta runs is what drives the highlight states on RegattaCentral's
// display pages.
type RaceStatus string

const (
	StatusPreDraw             RaceStatus = "PreDraw"
	StatusDraw                RaceStatus = "Draw"
	StatusRacing              RaceStatus = "Racing"
	StatusStopped             RaceStatus = "Stopped"
	StatusStoppedCollision    RaceStatus = "StoppedCollision"
	StatusStoppedFalseStart   RaceStatus = "StoppedFalseStart"
	StatusStoppedStartZoneDmg RaceStatus = "StoppedStartZoneDamage"
	StatusUnofficial          RaceStatus = "Unofficial"
	StatusOfficial            RaceStatus = "Official"
	StatusProtest             RaceStatus = "Protest"
	StatusRemovedCancelled    RaceStatus = "RemovedCancelled"
	StatusRemovedConsolidated RaceStatus = "RemovedConsolidated"
	StatusRemovedNonEvent     RaceStatus = "RemovedNonEvent"
	StatusUnknown             RaceStatus = "Unknown"
)

// LaneStatus is a per-entry status on a lane record (Cookbook §15).
type LaneStatus string

const (
	LaneOK           LaneStatus = ""
	LaneScratched    LaneStatus = "SCR"
	LaneDidNotStart  LaneStatus = "DNS"
	LaneDidNotFinish LaneStatus = "DNF"
	LaneDisqualified LaneStatus = "DSQ"
	LaneRemoved      LaneStatus = "RMV"
	LaneExcluded     LaneStatus = "EXC"
	LaneNotJudged    LaneStatus = "NJ"
	LaneByInvitation LaneStatus = "INV"
)

// Timing milestones (Cookbook §14): id 0 is always the start line, id 4 is
// always the finish; 1-3 are optional mid-course splits.
const (
	MilestoneStart  = 0
	MilestoneFinish = 4
)

// UploadRequest is the combined body PUT to /regattas/{id}/upload. Fields are
// ordered so lanes marshal before results, honouring "lanes MUST be reported
// prior to results" (Cookbook §14). Every field is optional; a request carries
// only what changed, and re-uploading overwrites.
type UploadRequest struct {
	// Flush true clears the regatta's race schedule, draws and results before
	// applying this body. Registration/entry data is untouched (Cookbook §10).
	Flush bool `json:"flush,omitempty"`

	Races   []RaceRecord   `json:"races,omitempty"`
	Lanes   []LaneRecord   `json:"lanes,omitempty"`
	Results []ResultRecord `json:"results,omitempty"`
}

// RaceRecord sets a race's status (and, per-race, an optional flush).
type RaceRecord struct {
	RaceID     int        `json:"raceId,omitempty"`
	RaceNumber int        `json:"raceNumber"`
	Status     RaceStatus `json:"status,omitempty"`
	Flush      bool       `json:"flush,omitempty"`
}

// LaneRecord is one boat's lane assignment for a race. EntryID references an
// existing RegattaCentral entry; UUID is set instead for an entry this client
// created locally with id 0.
type LaneRecord struct {
	RaceNumber    int        `json:"raceNumber"`
	Lane          int        `json:"lane"`
	DisplayNumber string     `json:"displayNumber,omitempty"`
	EntryID       int        `json:"entryId,omitempty"`
	UUID          string     `json:"uuid,omitempty"`
	Status        LaneStatus `json:"status,omitempty"`
}

// ResultRecord is timing data for one race/lane/milestone. Crew and event are
// deliberately absent - the key is race number + lane number (Cookbook §14).
// Time units are PROVISIONAL (milliseconds assumed).
type ResultRecord struct {
	RaceNumber        int   `json:"raceNumber"`
	Lane              int   `json:"lane"`
	TimingMilestoneID int   `json:"timingMilestoneId"`
	Time              int64 `json:"time,omitempty"`         // cumulative
	SplitTime         int64 `json:"splitTime,omitempty"`    // since previous split
	AdjustedTime      int64 `json:"adjustedTime,omitempty"` // cumulative w/ handicaps+penalties
}

// SetRaceStatus appends (or updates) a race status record.
func (u *UploadRequest) SetRaceStatus(raceNumber int, status RaceStatus) {
	for i := range u.Races {
		if u.Races[i].RaceNumber == raceNumber {
			u.Races[i].Status = status
			return
		}
	}
	u.Races = append(u.Races, RaceRecord{RaceNumber: raceNumber, Status: status})
}

// AddLane appends a lane record.
func (u *UploadRequest) AddLane(l LaneRecord) { u.Lanes = append(u.Lanes, l) }

// AddFinish appends a finish-line result for a race/lane.
func (u *UploadRequest) AddFinish(raceNumber, lane int, cumulative time.Duration) {
	u.Results = append(u.Results, ResultRecord{
		RaceNumber:        raceNumber,
		Lane:              lane,
		TimingMilestoneID: MilestoneFinish,
		Time:              cumulative.Milliseconds(),
	})
}

// Validate enforces the lanes-before-results rule within one request: every
// result's (raceNumber, lane) must also appear as a lane record here. Set
// assumeLanesUploaded to skip the check when the draw was uploaded earlier.
func (u *UploadRequest) Validate(assumeLanesUploaded bool) error {
	if assumeLanesUploaded || len(u.Results) == 0 {
		return nil
	}
	type key struct{ race, lane int }
	lanes := make(map[key]struct{}, len(u.Lanes))
	for _, l := range u.Lanes {
		lanes[key{l.RaceNumber, l.Lane}] = struct{}{}
	}
	for _, r := range u.Results {
		if _, ok := lanes[key{r.RaceNumber, r.Lane}]; !ok {
			return fmt.Errorf("regattacentral: result for race %d lane %d has no lane record in this upload (Cookbook §14: lanes MUST precede results)",
				r.RaceNumber, r.Lane)
		}
	}
	return nil
}
