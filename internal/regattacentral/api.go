package regattacentral

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// The read methods return raw JSON until the model is confirmed (see model.go).
// Paths follow api.regattacentral.com/v4/apiV4.jsp. A "" regattaID / eventID
// falls back to Config.RegattaID; an unset regatta id is ErrNoRegattaID.

// Token returns a currently-valid access token, acquiring or refreshing one if
// needed. It is exposed mainly for diagnostics (cmd/rcprobe) - normal callers
// use the API methods, which manage the token themselves.
func (c *Client) Token(ctx context.Context) (string, error) {
	return c.token.token(ctx)
}

// Bulk fetches the entire regatta - events, entries, athletes, organizations -
// in one response. Requires staff access. There is no incremental variant.
func (c *Client) Bulk(ctx context.Context, regattaID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "", "bulk")
}

// Events lists the events configured for the regatta.
func (c *Client) Events(ctx context.Context, regattaID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "", "events")
}

// EventEntries lists the entries in one event.
func (c *Client) EventEntries(ctx context.Context, regattaID, eventID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "", "events", eventID, "entries")
}

// EventLanes returns the lane draw / bow numbers for one event.
func (c *Client) EventLanes(ctx context.Context, regattaID, eventID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "", "events", eventID, "lanes")
}

// EventRaces lists the races for one event.
func (c *Client) EventRaces(ctx context.Context, regattaID, eventID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "", "events", eventID, "races")
}

// EventResults returns races, lanes and results for one event.
func (c *Client) EventResults(ctx context.Context, regattaID, eventID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "", "events", eventID, "results")
}

// ActiveRaces returns the events whose races are currently "In Progress".
func (c *Client) ActiveRaces(ctx context.Context, regattaID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "active", "races")
}

// Organizations lists the clubs / teams participating in the regatta.
func (c *Client) Organizations(ctx context.Context, regattaID string) (json.RawMessage, error) {
	return c.getRaw(ctx, regattaID, "", "organizations")
}

// SearchOrganizations does a partial-name lookup across the RegattaCentral
// organization database (not scoped to a regatta).
func (c *Client) SearchOrganizations(ctx context.Context, name string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodGet, "organizations", url.Values{"name": {name}}.Encode(), nil, &out)
	return out, err
}

// SearchParticipants does an exact-last-name + birthdate lookup across the
// RegattaCentral athlete database. birthdate must be "yyyy-mm-dd".
func (c *Client) SearchParticipants(ctx context.Context, lastname, birthdate string) (json.RawMessage, error) {
	var out json.RawMessage
	q := url.Values{"lastname": {lastname}, "birthdate": {birthdate}}.Encode()
	err := c.do(ctx, http.MethodGet, "participants", q, nil, &out)
	return out, err
}

// Upload PUTs a combined body to /regattas/{id}/upload. It runs
// UploadRequest.Validate first.
func (c *Client) Upload(ctx context.Context, regattaID string, req *UploadRequest) error {
	id, err := c.regattaID(regattaID)
	if err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	return c.do(ctx, http.MethodPut, "regattas/"+url.PathEscape(id)+"/upload", "", req, nil)
}

// CreateRaces POSTs new races to an existing event and returns them with
// RegattaCentral's real, newly-assigned RaceId populated. This is a
// dedicated per-resource creation endpoint, separate from Upload - RC's own
// REST API docs (api.regattacentral.com/v4/post.jsp#rc-post-event-races,
// read as raw HTML, not summarized) document it as returning "a collection
// of newly created Races, including the newly assigned RaceId." The
// response is decoded through the same {success,count,data,...} envelope
// every other confirmed RC v4 endpoint uses (see readmodel.go's Envelope).
//
// Uses "regattas" (plural), matching every other method in this file -
// RC's own docs consistently showed a singular "regatta" for this endpoint
// (checked via three independent raw-HTML extractions), but that turned out
// to be a real documentation bug: a live POST against "regatta/{id}/..."
// 404'd, while the identical request against "regattas/{id}/..." reached
// the real controller and returned a Jackson deserialization error instead
// (confirmed empirically by the author against the live API). The request
// body is a bare JSON array - that same live error named the exact
// deserialization target, `ArrayList<Race>`, confirming no wrapper object
// is expected.
func (c *Client) CreateRaces(ctx context.Context, regattaID, eventID string, races []RaceRecord) ([]RaceRecord, error) {
	id, err := c.regattaID(regattaID)
	if err != nil {
		return nil, err
	}
	path := "regattas/" + url.PathEscape(id) + "/events/" + url.PathEscape(eventID) + "/races"
	var resp Envelope[[]RaceRecord]
	if err := c.do(ctx, http.MethodPost, path, "", races, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("regattacentral: create races for event %s: request returned success=false", eventID)
	}
	return resp.Data, nil
}

// AssignLanes POSTs lane assignments to an already-created race, identified
// by its real RaceId (see CreateRaces), and returns the assigned lanes.
// Uses "regattas" (plural) - see CreateRaces.
func (c *Client) AssignLanes(ctx context.Context, regattaID string, raceID int, lanes []LaneRecord) ([]LaneRecord, error) {
	id, err := c.regattaID(regattaID)
	if err != nil {
		return nil, err
	}
	path := "regattas/" + url.PathEscape(id) + "/races/" + strconv.Itoa(raceID) + "/lanes"
	var resp Envelope[[]LaneRecord]
	if err := c.do(ctx, http.MethodPost, path, "", lanes, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("regattacentral: assign lanes for race %d: request returned success=false", raceID)
	}
	return resp.Data, nil
}

// getRaw resolves the regatta id, joins "regattas/{id}/<segs...>" and GETs it.
func (c *Client) getRaw(ctx context.Context, regattaID, rawQuery string, segs ...string) (json.RawMessage, error) {
	id, err := c.regattaID(regattaID)
	if err != nil {
		return nil, err
	}
	parts := make([]string, 0, len(segs)+2)
	parts = append(parts, "regattas", url.PathEscape(id))
	for _, s := range segs {
		parts = append(parts, url.PathEscape(strings.TrimSpace(s)))
	}
	var out json.RawMessage
	if err := c.do(ctx, http.MethodGet, strings.Join(parts, "/"), rawQuery, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
