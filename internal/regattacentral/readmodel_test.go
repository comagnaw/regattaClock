package regattacentral

import (
	"encoding/json"
	"testing"
)

// Every fixture below is hand-authored, using the confirmed real field
// names against made-up values - never a copy of a real capture (see the
// PII rule in docs/features/personas/heatsheet-rc-pivot-investigation.md).

func TestDecodeBulkNestsEventsAndEntries(t *testing.T) {
	raw := json.RawMessage(`{
		"success": true,
		"count": 1,
		"data": {
			"events": [
				{
					"eventId": 3,
					"title": "Men's Junior 1x",
					"gender": "male",
					"coxed": false,
					"sweep": false,
					"entries": [
						{
							"entryId": 61,
							"eventId": 3,
							"organizationId": 42,
							"division": "Junior",
							"alternateTitle": "M-Jr-1x",
							"entryLabel": "A",
							"statusCode": "OK",
							"entryParticipants": [
								{"participantId": 100, "entryId": 61, "name": "Alex Mihalovich"}
							]
						}
					]
				}
			],
			"organizations": [
				{"organizationId": 42, "name": "Springfield High School", "abbreviation": "SHS"}
			]
		}
	}`)

	resp, err := DecodeBulk(raw)
	if err != nil {
		t.Fatalf("DecodeBulk: %v", err)
	}
	if len(resp.Data.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(resp.Data.Events))
	}
	ev := resp.Data.Events[0]
	if ev.ID != "3" || ev.Title != "Men's Junior 1x" || ev.Gender != "male" {
		t.Errorf("event = %+v", ev)
	}
	if len(ev.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(ev.Entries))
	}
	e := ev.Entries[0]
	if e.ID != "61" || e.EventID != "3" || e.OrganizationID != "42" {
		t.Errorf("entry ids = %+v", e)
	}
	if e.Division != "Junior" || e.AlternateTitle != "M-Jr-1x" || e.Label != "A" || e.StatusCode != "OK" {
		t.Errorf("entry fields = %+v", e)
	}
	if len(e.Participants) != 1 || e.Participants[0].Name != "Alex Mihalovich" || e.Participants[0].ID != "100" {
		t.Errorf("participants = %+v", e.Participants)
	}
	if len(resp.Data.Organizations) != 1 || resp.Data.Organizations[0].ID != "42" ||
		resp.Data.Organizations[0].Name != "Springfield High School" || resp.Data.Organizations[0].Abbreviation != "SHS" {
		t.Errorf("organizations = %+v", resp.Data.Organizations)
	}
}

func TestDecodeEntriesFlatArray(t *testing.T) {
	raw := json.RawMessage(`{
		"success": true,
		"count": 2,
		"data": [
			{"entryId": "1", "eventId": "10", "organizationId": "5"},
			{"entryId": 2, "eventId": 10, "organizationId": 5}
		]
	}`)

	resp, err := DecodeEntries(raw)
	if err != nil {
		t.Fatalf("DecodeEntries: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("entries = %d, want 2", len(resp.Data))
	}
	// Same ids, one shape sent as JSON strings and the other as bare JSON
	// numbers - a real, observed inconsistency FlexibleID exists to absorb.
	if resp.Data[0].ID != "1" || resp.Data[1].ID != "2" {
		t.Errorf("ids = %q, %q, want \"1\", \"2\"", resp.Data[0].ID, resp.Data[1].ID)
	}
	if resp.Data[0].EventID != "10" || resp.Data[1].EventID != "10" {
		t.Errorf("eventIds = %q, %q, want both \"10\"", resp.Data[0].EventID, resp.Data[1].EventID)
	}
}

func TestDecodeOrganizationsFlatArray(t *testing.T) {
	raw := json.RawMessage(`{
		"success": true,
		"count": 1,
		"data": [
			{"organizationId": 7, "name": "Shelbyville Rowing Club"}
		]
	}`)

	resp, err := DecodeOrganizations(raw)
	if err != nil {
		t.Fatalf("DecodeOrganizations: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "7" || resp.Data[0].Name != "Shelbyville Rowing Club" {
		t.Errorf("organizations = %+v", resp.Data)
	}
	// ShortName/Abbreviation are absent from this fixture on purpose - a
	// real organization has been observed with abbreviation but never
	// shortName; both must decode to blank rather than erroring.
	if resp.Data[0].ShortName != "" || resp.Data[0].Abbreviation != "" {
		t.Errorf("organization = %+v, want blank ShortName/Abbreviation when absent", resp.Data[0])
	}
}

func TestDecodeEventsFlatArray(t *testing.T) {
	raw := json.RawMessage(`{
		"success": true,
		"count": 1,
		"data": [
			{"eventId": 20, "title": "Women's Junior 2x", "gender": "female", "coxed": false, "sweep": false}
		]
	}`)

	resp, err := DecodeEvents(raw)
	if err != nil {
		t.Fatalf("DecodeEvents: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "20" || resp.Data[0].Title != "Women's Junior 2x" {
		t.Errorf("events = %+v", resp.Data)
	}
}

func TestFlexibleIDRejectsNonIDValue(t *testing.T) {
	var id FlexibleID
	if err := json.Unmarshal([]byte(`{"not": "an id"}`), &id); err == nil {
		t.Error("want an error decoding an object into a FlexibleID")
	}
}

func TestDecodeBulkRejectsMalformedJSON(t *testing.T) {
	if _, err := DecodeBulk([]byte(`not json`)); err == nil {
		t.Error("want an error decoding malformed JSON")
	}
}
