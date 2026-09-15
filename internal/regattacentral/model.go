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
// typed, because the Cookbook's prose and RC's own hosted XSD schema docs pin
// down the field and enum names - though a live 404 already forced one full
// reshape (see the investigation doc's redesign entry) and the split-race
// behavior below is still PROVISIONAL until a round-trip against the live API
// confirms it.

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

// LaneStatus is a per-entry status on a lane record. Confirmed directly
// against the official RegattaCentral API V4.0 Cookbook PDF (§15, Lane
// Draw's own quoted status table): "SCR", "DNS", "DNF", "DSQ", "RMV", "EXC",
// "NJ", "INV". An earlier pass "corrected" LaneDisqualified from this
// original, correct "DSQ" to "DQ", based on an AI-summarized fetch of
// api.regattacentral.com/v4/xsd_doc/resultstatustype.html - that page was
// very likely showing the *Java enum constant name* from the Cookbook's own
// generated-code example (`ResultStatusType.DQ`, in its §4 LaneConstructor
// sample), not the wire value; the Cookbook's own explicit quoted-string
// table is the more authoritative, primary source, and "DSQ" is restored.
// EXH is independently confirmed by that same LaneConstructor example
// (`case EXHIBITION_LITERAL : this.status = ResultStatusType.EXH`), so it
// stays. The literal "OK" value is not modeled - LaneOK stays the empty
// string (an omitted field), the standard REST default-state convention,
// until there's a reason to believe RC actually requires the literal "OK".
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
	// LaneExhibition marks a crew racing outside official competition - not
	// scoring, not counted in results - the real case that motivated
	// confirming this enum: an RD can combine a small class into another
	// race's open lanes for lack of entries (see
	// heatsheet-rc-pivot-investigation.md) and mark that boat "Exhibition"
	// on the Heat Sheet tab.
	LaneExhibition LaneStatus = "EXH"
	// LaneRelegated marks a crew moved down from its original heat/division
	// (a rowing-specific meaning confirmed by the author).
	LaneRelegated LaneStatus = "REL"
)

// Timing milestones (Cookbook §14): id 0 is always the start line, id 4 is
// always the finish; 1-3 are optional mid-course splits.
const (
	MilestoneStart  = 0
	MilestoneFinish = 4
)

// UploadRequest is the combined body PUT to /regattas/{id}/upload. The real
// wire shape (confirmed via RC's hosted xsd_doc pages, cross-checked against
// the Cookbook PDF's own prose - see the investigation doc) is a nested tree:
// events -> races -> lanes -> results, not the flat sibling arrays this type
// used before a real 404 forced this reshape. Every field is optional; a
// request carries only what changed, and re-uploading overwrites (Cookbook
// §13).
type UploadRequest struct {
	// Flush true clears the regatta's race schedule, draws and results before
	// applying this body. Registration/entry data is untouched (Cookbook §10).
	Flush bool `json:"flush,omitempty"`

	Events []EventRecord `json:"events,omitempty"`
}

// EventRecord is one EXISTING RegattaCentral event and the races published
// under it. Every event this tool publishes to already exists - confirmed via
// a real bulk.json/events.json capture showing every event carries a
// non-zero eventId - so there is no event-creation path here at all (unlike
// Cookbook §11's "assign a UUID, RC allocates the ID" flow for a genuinely
// new event). A zero EventID is always a caller bug, never "create a new
// event".
type EventRecord struct {
	EventID int          `json:"eventId"`
	Races   []RaceRecord `json:"races,omitempty"`
}

// RaceRecord is one race nested under its parent EventRecord, used both as
// the request/response element type for the dedicated Client.CreateRaces
// endpoint and as part of Client.Upload's nested body. On a CreateRaces
// request, RaceID stays 0 and UUID is set to a freshly-generated id - the
// same "assign a UUID for a new entity" convention Cookbook page 2
// documents generally ("When the timing system creates a new entity, it
// must assign a UUID and use this UUID whenever that entity is
// referenced"), applied here to races too even though §11's worked example
// only spells it out for events. DisplayNumber additionally correlates a
// request race to its response, since nothing guarantees the response
// array preserves request order. Once CreateRaces returns, RaceID is the
// REAL id RegattaCentral assigned, and that real id - not the UUID, not the
// xlsm race number - is what every subsequent Upload call must use to
// reference this race (Cookbook §13: "Races and results are maintained
// using the raceId which is maintained on the timing system" - the timing
// system's obligation is to remember and reuse the real id CreateRaces gave
// it, mirroring how it already does this for Event/Entry ids). The UUID is
// single-use: nothing ever needs to look a race up by it again once its
// real id is known, unlike an Entry's UUID (Cookbook §12), which stays the
// entry's reference until RC allocates its id.
//
// EventID is kept equal to the parent EventRecord's EventID - set by the
// find-or-create helpers below, never independently settable, so it can't
// drift out of sync with its own nesting.
//
// The SAME xlsm race number can legitimately map to more than one RaceRecord
// across different EventRecords: an RD can combine a small boat class into
// another class's race for lack of entries, so a single physical race (one
// xlsm race number) can contain lanes belonging to more than one RC event.
// Each event-scoped portion is created independently via CreateRaces and
// gets its own distinct real RaceID - there is no shared identifier between
// them once creation is a real, separate RC operation per event.
type RaceRecord struct {
	RaceID        int          `json:"raceId"`
	EventID       int          `json:"eventId,omitempty"`
	UUID          string       `json:"uuid,omitempty"`
	DisplayNumber string       `json:"displayNumber,omitempty"`
	Status        RaceStatus   `json:"status,omitempty"`
	Flush         bool         `json:"flush,omitempty"`
	Lanes         []LaneRecord `json:"lanes,omitempty"`
}

// LaneRecord is one boat's lane assignment, nested under its parent
// RaceRecord. EntryID references an existing RegattaCentral entry; UUID is
// set instead for an entry this client created locally with id 0. Results
// nests this lane's timing data - a ResultRecord cannot exist without a
// parent LaneRecord in this shape.
type LaneRecord struct {
	Lane          int            `json:"lane"`
	DisplayNumber string         `json:"displayNumber,omitempty"`
	EntryID       int            `json:"entryId,omitempty"`
	UUID          string         `json:"uuid,omitempty"`
	Status        LaneStatus     `json:"status,omitempty"`
	Results       []ResultRecord `json:"results,omitempty"`
}

// ResultRecord is timing data for one milestone, nested under its parent
// LaneRecord - race/lane identity now comes entirely from the tree position.
// Field names are the Cookbook §14 prose's own literally-quoted tag names
// ("timingMilestoneId"/"time"/"splitTime"/"adjustedTime"), trusted over RC's
// xsd_doc pages, which list a different field set (splitLocation/
// elapsedTime/...) for this one type. Given this investigation's own DSQ/DQ
// precedent (an AI-summarized xsd_doc fetch once "corrected" a real wire
// value to a Java enum constant name that was never actually sent on the
// wire), the Cookbook's own quoted prose wins where the two sources
// disagree - see the investigation doc. Time units are PROVISIONAL
// (milliseconds assumed).
type ResultRecord struct {
	TimingMilestoneID int   `json:"timingMilestoneId"`
	Time              int64 `json:"time,omitempty"`         // cumulative
	SplitTime         int64 `json:"splitTime,omitempty"`    // since previous split
	AdjustedTime      int64 `json:"adjustedTime,omitempty"` // cumulative w/ handicaps+penalties
}

// findOrCreateEvent returns the EventRecord for eventID, appending a new,
// empty one if this is the first reference to it.
func (u *UploadRequest) findOrCreateEvent(eventID int) *EventRecord {
	for i := range u.Events {
		if u.Events[i].EventID == eventID {
			return &u.Events[i]
		}
	}
	u.Events = append(u.Events, EventRecord{EventID: eventID})
	return &u.Events[len(u.Events)-1]
}

// findOrCreateRace returns the RaceRecord for raceID within e, appending a
// new one (with EventID already set to e's own) if this is the first
// reference to it.
func (e *EventRecord) findOrCreateRace(raceID int) *RaceRecord {
	for i := range e.Races {
		if e.Races[i].RaceID == raceID {
			return &e.Races[i]
		}
	}
	e.Races = append(e.Races, RaceRecord{RaceID: raceID, EventID: e.EventID})
	return &e.Races[len(e.Races)-1]
}

// findOrCreateLane returns the LaneRecord for lane within r, appending a new,
// minimal one if this is the first reference to it.
func (r *RaceRecord) findOrCreateLane(lane int) *LaneRecord {
	for i := range r.Lanes {
		if r.Lanes[i].Lane == lane {
			return &r.Lanes[i]
		}
	}
	r.Lanes = append(r.Lanes, LaneRecord{Lane: lane})
	return &r.Lanes[len(r.Lanes)-1]
}

// SetRaceStatus sets (creating the event/race shell if needed) one race's
// display number and status, scoped to eventID - the same xlsm race number
// can legitimately appear under more than one EventRecord (see RaceRecord's
// doc comment), so eventID is part of the key, not just raceID.
func (u *UploadRequest) SetRaceStatus(eventID, raceID int, displayNumber string, status RaceStatus) {
	race := u.findOrCreateEvent(eventID).findOrCreateRace(raceID)
	race.DisplayNumber = displayNumber
	race.Status = status
}

// AddLane appends a lane record under (eventID, raceID), creating the
// EventRecord/RaceRecord shell if this is the first lane seen for that pair.
// Calling this with the same raceID but a different eventID (the
// mixed-boat-class case) is expected and correct: it produces two
// RaceRecords, nested under two different EventRecords, sharing RaceID -
// each holding only the lanes that belong to its own event.
func (u *UploadRequest) AddLane(eventID, raceID int, l LaneRecord) {
	race := u.findOrCreateEvent(eventID).findOrCreateRace(raceID)
	race.Lanes = append(race.Lanes, l)
}

// AddFinish appends a finish-line result to the lane identified by
// (eventID, raceID, lane), creating a minimal shell LaneRecord for it if
// AddLane wasn't already called for that exact lane.
func (u *UploadRequest) AddFinish(eventID, raceID, lane int, cumulative time.Duration) {
	l := u.findOrCreateEvent(eventID).findOrCreateRace(raceID).findOrCreateLane(lane)
	l.Results = append(l.Results, ResultRecord{
		TimingMilestoneID: MilestoneFinish,
		Time:              cumulative.Milliseconds(),
	})
}

// Validate is a cheap client-side sanity net worth keeping specifically
// because this is a live, hard-to-reverse write. The old flat shape's "lanes
// MUST precede results" check (Cookbook §14) is now structurally impossible
// to violate - a ResultRecord cannot exist without a parent LaneRecord in
// this nested shape - so Validate instead catches the mistakes nesting makes
// possible: a zero EventID (every event this tool publishes to must already
// exist - see EventRecord's doc comment), a zero RaceID, a RaceID repeated
// WITHIN one EventRecord (a real bug), and a lane number repeated within one
// race. The same RaceID repeating ACROSS different EventRecords is the
// intentional mixed-boat-class split and is explicitly allowed.
func (u *UploadRequest) Validate() error {
	for _, ev := range u.Events {
		if ev.EventID == 0 {
			return fmt.Errorf("regattacentral: event record with eventId 0 - every event this tool publishes to must already exist on RegattaCentral")
		}
		seenRace := map[int]bool{}
		for _, race := range ev.Races {
			if race.RaceID == 0 {
				return fmt.Errorf("regattacentral: event %d: race record with raceId 0", ev.EventID)
			}
			if seenRace[race.RaceID] {
				return fmt.Errorf("regattacentral: event %d: duplicate raceId %d within one event", ev.EventID, race.RaceID)
			}
			seenRace[race.RaceID] = true

			seenLane := map[int]bool{}
			for _, lane := range race.Lanes {
				if seenLane[lane.Lane] {
					return fmt.Errorf("regattacentral: event %d race %d: duplicate lane %d", ev.EventID, race.RaceID, lane.Lane)
				}
				seenLane[lane.Lane] = true
			}
		}
	}
	return nil
}
