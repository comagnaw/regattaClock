package regattacentral

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func testCreds() Credentials {
	return Credentials{ClientID: "cid", ClientSecret: "csec", Username: "user@example.com", Password: "pw"}
}

// tokenServer serves the OAuth2 token endpoint, recording every form it
// receives and replying with scripted token responses.
type tokenServer struct {
	mu      sync.Mutex
	forms   []url.Values
	replies []string // JSON bodies, consumed in order; last one repeats
	status  []int    // parallel to replies; 0 -> 200
}

func (s *tokenServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))

		s.mu.Lock()
		s.forms = append(s.forms, form)
		i := len(s.forms) - 1
		if i >= len(s.replies) {
			i = len(s.replies) - 1
		}
		reply := s.replies[i]
		code := 200
		if i < len(s.status) && s.status[i] != 0 {
			code = s.status[i]
		}
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		io.WriteString(w, reply)
	}
}

func (s *tokenServer) lastForm() url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forms[len(s.forms)-1]
}

func (s *tokenServer) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.forms)
}

func TestTokenAcquirePasswordGrant(t *testing.T) {
	ts := &tokenServer{replies: []string{`{"access_token":"tok1","refresh_token":"r1","expires_in":3600}`}}
	srv := httptest.NewServer(ts.handler())
	defer srv.Close()

	c, err := New(Config{Credentials: testCreds(), TokenURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.token.token(context.Background())
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if got != "tok1" {
		t.Fatalf("token = %q, want tok1", got)
	}

	f := ts.lastForm()
	for k, want := range map[string]string{
		"grant_type": "password", "client_id": "cid", "client_secret": "csec",
		"username": "user@example.com", "password": "pw",
	} {
		if f.Get(k) != want {
			t.Errorf("form[%s] = %q, want %q", k, f.Get(k), want)
		}
	}

	// A second call inside the validity window does not hit the endpoint again.
	if _, err := c.token.token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ts.calls() != 1 {
		t.Errorf("token endpoint called %d times, want 1 (cached)", ts.calls())
	}
}

func TestTokenRefreshOnExpiry(t *testing.T) {
	ts := &tokenServer{replies: []string{
		`{"access_token":"tok1","refresh_token":"r1","expires_in":120}`,
		`{"access_token":"tok2","refresh_token":"r2","expires_in":3600}`,
	}}
	srv := httptest.NewServer(ts.handler())
	defer srv.Close()

	now := time.Now()
	c, err := New(Config{Credentials: testCreds(), TokenURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	c.cfg.now = func() time.Time { return now }
	// token source captured &c.cfg, so mutating c.cfg.now is visible to it.

	if got, _ := c.token.token(context.Background()); got != "tok1" {
		t.Fatalf("first token = %q", got)
	}
	now = now.Add(200 * time.Second) // past expiry (120s) and skew

	got, err := c.token.token(context.Background())
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got != "tok2" {
		t.Fatalf("refreshed token = %q, want tok2", got)
	}
	if f := ts.lastForm(); f.Get("grant_type") != "refresh_token" || f.Get("refresh_token") != "r1" {
		t.Errorf("refresh form = %v, want grant_type=refresh_token refresh_token=r1", f)
	}
}

func TestTokenRefreshFallsBackToPassword(t *testing.T) {
	ts := &tokenServer{
		replies: []string{
			`{"access_token":"tok1","refresh_token":"r1","expires_in":120}`,
			`{"error":"invalid_grant"}`,
			`{"access_token":"tok3","refresh_token":"r3","expires_in":3600}`,
		},
		status: []int{200, 400, 200},
	}
	srv := httptest.NewServer(ts.handler())
	defer srv.Close()

	now := time.Now()
	c, _ := New(Config{Credentials: testCreds(), TokenURL: srv.URL, HTTPClient: srv.Client()})
	c.cfg.now = func() time.Time { return now }

	c.token.token(context.Background())
	now = now.Add(200 * time.Second)

	got, err := c.token.token(context.Background())
	if err != nil {
		t.Fatalf("token after failed refresh: %v", err)
	}
	if got != "tok3" {
		t.Fatalf("token = %q, want tok3 (password fallback)", got)
	}
	if ts.calls() != 3 {
		t.Fatalf("token endpoint calls = %d, want 3 (initial, failed refresh, password)", ts.calls())
	}
	if ts.lastForm().Get("grant_type") != "password" {
		t.Errorf("final grant_type = %q, want password", ts.lastForm().Get("grant_type"))
	}
}

func TestTokenAuthErrorNotRetried(t *testing.T) {
	ts := &tokenServer{replies: []string{`{"error":"invalid_client"}`}, status: []int{401}}
	srv := httptest.NewServer(ts.handler())
	defer srv.Close()

	c, _ := New(Config{Credentials: testCreds(), TokenURL: srv.URL, HTTPClient: srv.Client()})
	_, err := c.token.token(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err = %v, want ErrAuth", err)
	}
	if !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("err should carry the server body: %v", err)
	}
}

func TestNewRejectsIncompleteCredentials(t *testing.T) {
	_, err := New(Config{Credentials: Credentials{ClientID: "only-this"}})
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}
}
