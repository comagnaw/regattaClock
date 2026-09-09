package version

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetDefaults(t *testing.T) {
	c := Get()
	// A `go test` build has no -ldflags, so every attribute is the dev sentinel.
	for _, kv := range c.Fields() {
		if kv[1] != dev {
			t.Errorf("%s = %q, want %q for an un-linked build", kv[0], kv[1], dev)
		}
	}
	if c.Source != dev {
		t.Errorf("Source = %q, want %q when nothing is linked in", c.Source, dev)
	}
}

func TestGetSourceUsesCommit(t *testing.T) {
	origURL, origCommit := RepoURL, BuildCommit
	t.Cleanup(func() { RepoURL, BuildCommit = origURL, origCommit })
	RepoURL = "https://example.test/acme/widget"
	BuildCommit = "abc1234"

	if got, want := Get().Source, "https://example.test/acme/widget/commit/abc1234"; got != want {
		t.Errorf("Source = %q, want %q", got, want)
	}
}

func TestGetSourceFallsBackToRepoRoot(t *testing.T) {
	origURL, origCommit := RepoURL, BuildCommit
	t.Cleanup(func() { RepoURL, BuildCommit = origURL, origCommit })
	RepoURL = "https://example.test/acme/widget"
	BuildCommit = dev

	if got := Get().Source; got != RepoURL {
		t.Errorf("Source = %q, want the bare repo URL %q when the commit is unknown", got, RepoURL)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	var back Current
	if err := json.Unmarshal([]byte(Get().JSON()), &back); err != nil {
		t.Fatalf("JSON() is not valid JSON: %v", err)
	}
	if back.Version != Version {
		t.Errorf("round-tripped version = %q, want %q", back.Version, Version)
	}
	if !strings.Contains(Get().JSON(), `"source"`) {
		t.Error(`JSON() output is missing the "source" field`)
	}
}

func TestFieldsShape(t *testing.T) {
	f := Get().Fields()
	if len(f) != 7 {
		t.Fatalf("Fields() has %d rows, want 7", len(f))
	}
	for _, kv := range f {
		if kv[0] == "" || kv[1] == "" {
			t.Errorf("empty pair in Fields(): %q => %q", kv[0], kv[1])
		}
	}
}
