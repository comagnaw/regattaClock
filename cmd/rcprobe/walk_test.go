package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

// recordingServer serves the token endpoint plus a fixed map of path -> body,
// recording every non-token path it was asked for.
type recordingServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newRecordingServer(t *testing.T, paths map[string]string) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/oauth2/api/token") {
			io.WriteString(w, `{"access_token":"tok","refresh_token":"r","expires_in":3600}`)
			return
		}
		rs.mu.Lock()
		rs.hits = append(rs.hits, r.URL.Path)
		rs.mu.Unlock()
		if body, ok := paths[r.URL.Path]; ok {
			io.WriteString(w, body)
			return
		}
		http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
	}))
	t.Cleanup(rs.Close)
	return rs
}

func clientAgainst(t *testing.T, srv *httptest.Server) *regattacentral.Client {
	t.Helper()
	c, err := regattacentral.New(regattacentral.Config{
		Credentials: regattacentral.Credentials{ClientID: "c", ClientSecret: "s", Username: "u", Password: "p"},
		TokenURL:    srv.URL + "/oauth2/api/token",
		BaseURL:     srv.URL + "/v4.0/",
		HTTPClient:  srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRunWalk(t *testing.T) {
	bulk := `{"regatta":{"id":"R1"},"events":[{"id":42,"name":"Event A"},{"id":"77","name":"Event B"}]}`
	srv := newRecordingServer(t, map[string]string{
		"/v4.0/regattas/R1/bulk":              bulk,
		"/v4.0/regattas/R1/events/42/entries": `[{"id":1}]`,
		"/v4.0/regattas/R1/events/77/entries": `[{"id":2}]`,
	})
	client := clientAgainst(t, srv.Server)
	out := t.TempDir()

	if err := runWalk(context.Background(), client, "R1", out); err != nil {
		t.Fatalf("runWalk: %v", err)
	}

	for _, name := range []string{"bulk.json", "entries-42.json", "entries-77.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("expected %s to be written: %v", name, err)
		}
	}
}

func TestRunWalkNoEventsFound(t *testing.T) {
	srv := newRecordingServer(t, map[string]string{
		"/v4.0/regattas/R1/bulk": `{"regatta":{"id":"R1"}}`,
	})
	client := clientAgainst(t, srv.Server)
	out := t.TempDir()

	if err := runWalk(context.Background(), client, "R1", out); err != nil {
		t.Fatalf("runWalk with no events should not error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "bulk.json")); err != nil {
		t.Errorf("bulk.json should still be written: %v", err)
	}
}

func TestRunWalkOneEventFailsButOthersSucceed(t *testing.T) {
	srv := newRecordingServer(t, map[string]string{
		"/v4.0/regattas/R1/bulk":             `{"events":[{"id":1},{"id":2}]}`,
		"/v4.0/regattas/R1/events/2/entries": `[]`,
		// event 1's entries path is deliberately absent -> 404
	})
	client := clientAgainst(t, srv.Server)
	out := t.TempDir()

	err := runWalk(context.Background(), client, "R1", out)
	if err == nil || !strings.Contains(err.Error(), "1 of 2 event(s) failed") {
		t.Fatalf("err = %v, want a partial-failure summary", err)
	}
	if _, statErr := os.Stat(filepath.Join(out, "entries-2.json")); statErr != nil {
		t.Errorf("the event that succeeded should still be saved: %v", statErr)
	}
}

func TestRunWalkRequiresOutDir(t *testing.T) {
	if err := runWalk(context.Background(), nil, "R1", ""); err == nil {
		t.Fatal("expected an error when --out is empty")
	}
}

func TestEventIDsFromBulk(t *testing.T) {
	raw := json.RawMessage(`{
		"regatta": {"id": "R1", "name": "Test"},
		"events": [
			{"id": 42, "entries": [{"id": 1, "eventId": 42}]},
			{"id": "77"}
		]
	}`)
	ids, err := eventIDsFromBulk(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"42": true, "77": true}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want exactly %v", ids, want)
	}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected id %q", id)
		}
	}
}

func TestEventIDsFromBulkDoesNotConfuseEntryIDs(t *testing.T) {
	// An entry's own "id" must not be mistaken for an event id just because
	// the entry is nested inside an "events" array.
	raw := json.RawMessage(`{"events":[{"id":1,"entries":[{"id":999}]}]}`)
	ids, err := eventIDsFromBulk(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "1" {
		t.Fatalf("ids = %v, want [1] only (entry id 999 must not leak in)", ids)
	}
}

func TestEventIDsFromBulkDedupes(t *testing.T) {
	raw := json.RawMessage(`{"events":[{"id":1,"eventId":1},{"id":1}]}`)
	ids, err := eventIDsFromBulk(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("ids = %v, want exactly one", ids)
	}
}
