package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/comagnaw/regattaClock/internal/secretstore"
)

// stubServer serves the token endpoint and one API path.
func stubServer(t *testing.T, apiPath, apiBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/oauth2/api/token") {
			io.WriteString(w, `{"access_token":"tok","refresh_token":"r","expires_in":3600}`)
			return
		}
		if r.URL.Path == apiPath {
			io.WriteString(w, apiBody)
			return
		}
		http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRun_BulkWritesCapture(t *testing.T) {
	srv := stubServer(t, "/v4.0/regattas/R42/bulk", `{"regatta":{"id":"R42"}}`)
	t.Cleanup(func() { secretstore.SetBackend(nil) })

	for k, v := range map[string]string{
		"RC_CLIENT_ID": "c", "RC_CLIENT_SECRET": "s", "RC_USERNAME": "u", "RC_PASSWORD": "p",
	} {
		t.Setenv(k, v)
	}

	out := t.TempDir()
	err := run([]string{
		"--token-url", srv.URL + "/oauth2/api/token",
		"--base-url", srv.URL + "/v4.0/",
		"--regatta", "R42",
		"--out", out,
		"bulk",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(out, "bulk.json"))
	if err != nil {
		t.Fatalf("capture not written: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("capture is not valid JSON: %v", err)
	}
	if _, ok := got["regatta"]; !ok {
		t.Errorf("capture missing expected content: %s", b)
	}
}

func TestRun_MissingCredentials(t *testing.T) {
	t.Cleanup(func() { secretstore.SetBackend(nil) })
	for _, k := range []string{"RC_CLIENT_ID", "RC_CLIENT_SECRET", "RC_USERNAME", "RC_PASSWORD"} {
		t.Setenv(k, "")
	}
	err := run([]string{"bulk", "R1"})
	if err == nil || !strings.Contains(err.Error(), "client_id") {
		t.Fatalf("err = %v, want a missing-credential error", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	t.Cleanup(func() { secretstore.SetBackend(nil) })
	for k, v := range map[string]string{
		"RC_CLIENT_ID": "c", "RC_CLIENT_SECRET": "s", "RC_USERNAME": "u", "RC_PASSWORD": "p",
	} {
		t.Setenv(k, v)
	}
	if err := run([]string{"frobnicate"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("err = %v, want unknown command", err)
	}
}
