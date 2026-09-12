package regattacentral

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// This file is the read-side counterpart to model.go's typed write model:
// structs for what a real capture has confirmed, not what the Cookbook's
// prose implies. api.go's Bulk/EventEntries/Organizations/Events methods
// keep returning json.RawMessage unchanged - cmd/rcprobe depends on
// full-fidelity raw captures for its walk/shape commands, and a typed
// round-trip would silently drop every field not modeled here. These types
// exist for a caller (cmd/rcreconcile, or a future real integration) that
// wants a confirmed, typed view without re-deriving one from scratch.
//
// Every field below was confirmed by tracing one real entry's own JSON keys
// and cross-checking a dedicated organizations/events listing against a
// live capture (see docs/features/personas/heatsheet-rc-pivot-investigation.md) -
// not guessed from the Cookbook's prose alone. Only fields cmd/rcreconcile's
// matcher actually consumes are modeled; encoding/json silently ignores
// everything else a real object carries (Entry alone has ~20 more real
// fields - ageCategory, bow, composite, contactId, handicap, seed, venue,
// and so on - none of them needed for matching).

// FlexibleID decodes an id-shaped JSON field that RC sends as either a bare
// number or a quoted string (observed both ways across different real
// captures) into a plain decimal string - the same normalization
// cmd/rcreconcile's firstString helper did by hand before this file existed.
type FlexibleID string

func (id *FlexibleID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*id = FlexibleID(s)
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("id is neither a string nor a number: %s", b)
	}
	*id = FlexibleID(strconv.FormatFloat(f, 'f', -1, 64))
	return nil
}

// Participant is one rower/cox on an Entry (a real entryParticipants[]
// element).
type Participant struct {
	ID      FlexibleID `json:"participantId"`
	EntryID FlexibleID `json:"entryId"`
	Name    string     `json:"name"`
}

// Entry is a RegattaCentral crew registration.
type Entry struct {
	ID             FlexibleID    `json:"entryId"`
	EventID        FlexibleID    `json:"eventId"`
	OrganizationID FlexibleID    `json:"organizationId"`
	Division       string        `json:"division"`
	AlternateTitle string        `json:"alternateTitle"`
	Label          string        `json:"entryLabel"`
	StatusCode     string        `json:"statusCode"`
	Participants   []Participant `json:"entryParticipants"`
}

// Organization is a club/school/team. Its own id field is "organizationId",
// not the generic "id" a REST convention might suggest - confirmed against a
// real organizations listing, which had no bare "id" key at all.
// ShortName follows the same naming convention as Abbreviation but has not
// itself been confirmed present on a real object; it stays blank rather than
// erroring when absent.
type Organization struct {
	ID           FlexibleID `json:"organizationId"`
	Name         string     `json:"name"`
	ShortName    string     `json:"shortName"`
	Abbreviation string     `json:"abbreviation"`
}

// Event is one boat-class/gender/age grouping that entries and (eventually)
// races belong to. Races field is deliberately not modeled: every real
// capture seen so far has an empty races[] array for every event (RC's own
// scheduling was never used for this regatta), so its own shape remains
// unconfirmed - see heatsheet-rc-pivot-investigation.md's mixed-boat-class
// finding for why that matters.
type Event struct {
	ID           FlexibleID `json:"eventId"`
	Title        string     `json:"title"`
	Label        string     `json:"label"`
	Gender       string     `json:"gender"`
	Coxed        bool       `json:"coxed"`
	Sweep        bool       `json:"sweep"`
	Equipment    string     `json:"equipment"`
	AthleteClass string     `json:"athleteClass"`
	Entries      []Entry    `json:"entries"`
}

// Envelope is the shared response shape every confirmed RegattaCentral v4
// endpoint used by this package returns - bulk.json, organizations.json,
// events.json and entries-<id>.json all share this same top-level shape.
// Only Data's type differs per endpoint: a single nested object for bulk
// (BulkData), a flat array for the dedicated listings.
type Envelope[T any] struct {
	Success  bool            `json:"success"`
	Count    int             `json:"count"`
	Data     T               `json:"data"`
	Links    json.RawMessage `json:"links"`
	Messages json.RawMessage `json:"messages"`
}

// BulkData is bulk.json's nested "data" object - the whole regatta in one
// response. Only Events and Organizations are modeled; a real bulk response
// carries ~30 more regatta-level fields (billingAddress, contacts,
// deadlines, entryWindowOpens, venue, and so on) not needed for matching.
type BulkData struct {
	Events        []Event        `json:"events"`
	Organizations []Organization `json:"organizations"`
}

type (
	BulkResponse          = Envelope[BulkData]
	EntriesResponse       = Envelope[[]Entry]
	OrganizationsResponse = Envelope[[]Organization]
	EventsResponse        = Envelope[[]Event]
)

// DecodeBulk parses a /bulk response (see Client.Bulk) into its confirmed
// typed shape.
func DecodeBulk(raw json.RawMessage) (*BulkResponse, error) {
	var resp BulkResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode bulk response: %w", err)
	}
	return &resp, nil
}

// DecodeEntries parses an /events/{id}/entries response (see
// Client.EventEntries) into its confirmed typed shape.
func DecodeEntries(raw json.RawMessage) (*EntriesResponse, error) {
	var resp EntriesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode entries response: %w", err)
	}
	return &resp, nil
}

// DecodeOrganizations parses an /organizations response (see
// Client.Organizations) into its confirmed typed shape.
func DecodeOrganizations(raw json.RawMessage) (*OrganizationsResponse, error) {
	var resp OrganizationsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode organizations response: %w", err)
	}
	return &resp, nil
}

// DecodeEvents parses an /events response (see Client.Events) into its
// confirmed typed shape.
func DecodeEvents(raw json.RawMessage) (*EventsResponse, error) {
	var resp EventsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode events response: %w", err)
	}
	return &resp, nil
}
