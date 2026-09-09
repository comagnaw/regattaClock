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
	if c.Full != dev {
		t.Errorf("Full = %q, want %q for an un-linked build", c.Full, dev)
	}
	if c.Build != "" {
		t.Errorf("Build = %q, want empty for an un-linked build", c.Build)
	}
	if c.Dirty {
		t.Error("Dirty = true for an un-linked build")
	}
}

func TestBuildMetadata(t *testing.T) {
	restore := stub(t)
	defer restore()
	Version, BuildCommit, BuildCount, BuildDirty = "0.4.1-alpha", "c0ffee1", "127", "false"

	c := Get()
	if c.Build != "127.gc0ffee1" {
		t.Errorf("Build = %q, want %q", c.Build, "127.gc0ffee1")
	}
	if c.Full != "0.4.1-alpha+127.gc0ffee1" {
		t.Errorf("Full = %q, want %q", c.Full, "0.4.1-alpha+127.gc0ffee1")
	}
	if c.Version != "0.4.1-alpha" {
		t.Errorf("Version = %q, want the bare release line %q", c.Version, "0.4.1-alpha")
	}
}

func TestBuildExactTag(t *testing.T) {
	restore := stub(t)
	defer restore()
	Version, BuildCommit, BuildCount, BuildDirty = "0.4.1-alpha", "c0ffee1", "0", "false"

	c := Get()
	if c.Build != "" {
		t.Errorf("Build = %q, want empty on an exact tag", c.Build)
	}
	if c.Full != "0.4.1-alpha" {
		t.Errorf("Full = %q, want %q on an exact clean tag", c.Full, "0.4.1-alpha")
	}
}

func TestBuildDirty(t *testing.T) {
	restore := stub(t)
	defer restore()

	Version, BuildCommit, BuildCount, BuildDirty = "0.4.1-alpha", "c0ffee1", "0", "true"
	if got := Get().Full; got != "0.4.1-alpha+dirty" {
		t.Errorf("Full = %q, want %q for a dirty build on an exact tag", got, "0.4.1-alpha+dirty")
	}

	BuildCount = "5"
	if got := Get().Full; got != "0.4.1-alpha+5.gc0ffee1.dirty" {
		t.Errorf("Full = %q, want %q for a dirty build past a tag", got, "0.4.1-alpha+5.gc0ffee1.dirty")
	}
}

// stub saves the link-time vars this file rewrites and returns a func that puts
// them back. Every metadata test flips several at once, so a single helper beats
// per-test t.Cleanup closures.
func stub(t *testing.T) func() {
	t.Helper()
	v, c, n, d := Version, BuildCommit, BuildCount, BuildDirty
	return func() { Version, BuildCommit, BuildCount, BuildDirty = v, c, n, d }
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
