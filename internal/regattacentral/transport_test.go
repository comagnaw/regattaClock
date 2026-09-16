package regattacentral

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// testServer routes the token path to a rolling token issuer and everything
// else to the supplied API handler.
type testServer struct {
	*httptest.Server
	tokenN int
	mu     sync.Mutex
}

func newTestServer(t *testing.T, api http.HandlerFunc) *testServer {
	t.Helper()
	ts := &testServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/oauth2/api/token") {
			ts.mu.Lock()
			ts.tokenN++
			n := ts.tokenN
			ts.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"tok`+strconv.Itoa(n)+`","refresh_token":"r`+strconv.Itoa(n)+`","expires_in":3600}`)
			return
		}
		api(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func (ts *testServer) client(t *testing.T, cfg Config) *Client {
	t.Helper()
	cfg.Credentials = testCreds()
	cfg.TokenURL = ts.URL + "/oauth2/api/token"
	cfg.BaseURL = ts.URL + "/v4.0/"
	cfg.HTTPClient = ts.Server.Client()
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func recordingSleep() (*[]time.Duration, func(context.Context, time.Duration) error) {
	var got []time.Duration
	return &got, func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		got = append(got, d)
		return nil
	}
}

func TestDoRetriesOn503(t *testing.T) {
	var hits int
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, `{"ok":true}`)
	})
	waits, sleep := recordingSleep()
	c := ts.client(t, Config{sleep: sleep})

	out, err := c.Bulk(context.Background(), "R1")
	if err != nil {
		t.Fatalf("Bulk: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("body = %s", out)
	}
	if hits != 3 {
		t.Fatalf("api hits = %d, want 3", hits)
	}
	if len(*waits) != 2 || (*waits)[0] != 500*time.Millisecond || (*waits)[1] != time.Second {
		t.Fatalf("backoff waits = %v, want [500ms 1s]", *waits)
	}
}

func TestDoHonoursRetryAfter(t *testing.T) {
	var hits int
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		io.WriteString(w, `{}`)
	})
	waits, sleep := recordingSleep()
	c := ts.client(t, Config{sleep: sleep})

	if _, err := c.Events(context.Background(), "R1"); err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(*waits) != 1 || (*waits)[0] != 2*time.Second {
		t.Fatalf("waits = %v, want [2s] from Retry-After", *waits)
	}
}

func TestDoRefreshesTokenOn401(t *testing.T) {
	var (
		hits      int
		gotAuth   []string
		firstDone bool
	)
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		if !firstDone {
			firstDone = true
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"ok":1}`)
	})
	// A 401 triggers a token refresh + one retry regardless of the 429/5xx budget.
	c := ts.client(t, Config{})

	if _, err := c.Bulk(context.Background(), "R1"); err != nil {
		t.Fatalf("Bulk: %v", err)
	}
	if hits != 2 {
		t.Fatalf("api hits = %d, want 2 (401 then retry)", hits)
	}
	if gotAuth[0] == gotAuth[1] {
		t.Fatalf("token not rotated after 401: both %q", gotAuth[0])
	}
	if gotAuth[1] != "tok2" {
		t.Errorf("retry Authorization = %q, want tok2", gotAuth[1])
	}
}

func TestDoContextCancelStopsRetry(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ctx, cancel := context.WithCancel(context.Background())
	c := ts.client(t, Config{MaxRetries: 5, sleep: func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}})

	_, err := c.Bulk(ctx, "R1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestDoAPIErrorOn404(t *testing.T) {
	var hits int
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `not found`)
	})
	c := ts.client(t, Config{})

	_, err := c.Bulk(context.Background(), "R1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.StatusCode != 404 || hits != 1 {
		t.Fatalf("status=%d hits=%d, want 404 and 1 (no retry)", apiErr.StatusCode, hits)
	}
}

func TestDoUsesConfiguredRegattaID(t *testing.T) {
	var path string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		io.WriteString(w, `{}`)
	})
	c := ts.client(t, Config{RegattaID: "default-99"})

	if _, err := c.Bulk(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if path != "/v4.0/regattas/default-99/bulk" {
		t.Fatalf("path = %q", path)
	}

	if _, err := (&Client{cfg: Config{}}).regattaID(""); !errors.Is(err, ErrNoRegattaID) {
		t.Errorf("want ErrNoRegattaID when neither explicit nor config id set")
	}
}

func TestUploadPutsJSONBody(t *testing.T) {
	var (
		method, path, ctype string
		body                UploadRequest
	)
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, ctype = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	})
	c := ts.client(t, Config{})

	req := &UploadRequest{}
	req.SetRaceStatus(12, StatusDraw)
	req.AddLane(LaneRecord{RaceNumber: 12, Lane: 3, EntryID: 555})
	req.AddFinish(12, 3, 6*time.Minute+12500*time.Millisecond)

	if err := c.Upload(context.Background(), "R7", req, false); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if method != http.MethodPut || path != "/v4.0/regattas/R7/upload" {
		t.Fatalf("got %s %s", method, path)
	}
	if !strings.HasPrefix(ctype, "application/json") {
		t.Errorf("Content-Type = %q", ctype)
	}
	if len(body.Races) != 1 || body.Races[0].Status != StatusDraw {
		t.Errorf("races round-trip: %+v", body.Races)
	}
	if len(body.Results) != 1 || body.Results[0].TimingMilestoneID != MilestoneFinish || body.Results[0].Time != 372500 {
		t.Errorf("results round-trip: %+v", body.Results)
	}
}

func TestUploadValidatesLanesBeforeResults(t *testing.T) {
	var called bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	c := ts.client(t, Config{})

	req := &UploadRequest{}
	req.AddFinish(4, 2, time.Minute) // result with no matching lane

	err := c.Upload(context.Background(), "R1", req, false)
	if err == nil || !strings.Contains(err.Error(), "lanes MUST precede results") {
		t.Fatalf("err = %v, want a lanes-before-results validation error", err)
	}
	if called {
		t.Error("no HTTP request should be made when validation fails")
	}

	// assumeLanesUploaded bypasses the check.
	if err := c.Upload(context.Background(), "R1", req, true); err != nil {
		t.Fatalf("with assumeLanesUploaded: %v", err)
	}
}
