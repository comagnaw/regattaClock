package journal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type envelopeDoc struct {
	RegattaKey string
	Sequence   int
}

func mustWriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("could not create dir for %s: %v", path, err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("could not marshal %v: %v", v, err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("could not write %s: %v", path, err)
	}
}

func mustReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("could not unmarshal %s: %v", b, err)
	}
}

func TestRecoverPending_AdoptsLocalWriteThatNeverReachedShared(t *testing.T) {
	swapBackoff(t, time.Microsecond, time.Millisecond)

	dir := t.TempDir()
	sharedPath := filepath.Join(dir, "shared", "finish.json")
	localPath := filepath.Join(dir, "local", "finish.pending.json")

	// Simulate a crash: a local entry exists at Sequence 2, but the shared
	// file never got past Sequence 1 - the crash happened between the local
	// write and the shared write for the same Write call.
	mustWriteJSON(t, localPath, envelopeDoc{RegattaKey: "regatta-a", Sequence: 2})
	mustWriteJSON(t, sharedPath, envelopeDoc{RegattaKey: "regatta-a", Sequence: 1})

	m := newManager(sharedPath, localPath, "regatta-a")
	t.Cleanup(m.Close)

	b, ok := m.Recovered()
	if !ok {
		t.Fatal("expected a recovered entry")
	}
	var got envelopeDoc
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("could not unmarshal recovered bytes: %v", err)
	}
	if got.Sequence != 2 {
		t.Errorf("expected the recovered entry to be sequence 2, got %d", got.Sequence)
	}

	if _, ok := m.Recovered(); ok {
		t.Error("expected Recovered to report the finding only once")
	}

	waitForState(t, m, StateSynced, time.Second)
	var flushed envelopeDoc
	mustReadJSON(t, sharedPath, &flushed)
	if flushed.Sequence != 2 {
		t.Errorf("expected the shared file to catch up to sequence 2, got %d", flushed.Sequence)
	}
}

func TestRecoverPending_SharedAlreadyCaughtUp_RemovesLocalLeftover(t *testing.T) {
	dir := t.TempDir()
	sharedPath := filepath.Join(dir, "shared", "finish.json")
	localPath := filepath.Join(dir, "local", "finish.pending.json")

	mustWriteJSON(t, localPath, envelopeDoc{RegattaKey: "regatta-a", Sequence: 1})
	mustWriteJSON(t, sharedPath, envelopeDoc{RegattaKey: "regatta-a", Sequence: 1})

	m := newManager(sharedPath, localPath, "regatta-a")
	t.Cleanup(m.Close)

	if _, ok := m.Recovered(); ok {
		t.Error("expected nothing to recover once the shared file has caught up")
	}
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Errorf("expected the harmless local leftover to be removed, stat err = %v", err)
	}
	if got := m.Status().State; got != StateSynced {
		t.Errorf("expected Synced with nothing pending, got %s", got)
	}
}

func TestRecoverPending_DifferentRegattaKey_LeavesLocalEntryUntouched(t *testing.T) {
	dir := t.TempDir()
	sharedPath := filepath.Join(dir, "shared", "finish.json")
	localPath := filepath.Join(dir, "local", "finish.pending.json")

	mustWriteJSON(t, localPath, envelopeDoc{RegattaKey: "regatta-old", Sequence: 5})

	m := newManager(sharedPath, localPath, "regatta-new")
	t.Cleanup(m.Close)

	if _, ok := m.Recovered(); ok {
		t.Error("expected a different regatta's entry not to be recovered")
	}
	if got := m.Status().State; got != StateSynced {
		t.Errorf("expected Synced (nothing adopted), got %s", got)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Errorf("expected the mismatched entry to be left in place, stat err = %v", err)
	}
	if _, err := os.Stat(sharedPath); !os.IsNotExist(err) {
		t.Error("expected nothing written to the shared path for a mismatched regatta")
	}
}

func TestRecoverPending_NoLocalFile_NothingToRecover(t *testing.T) {
	dir := t.TempDir()
	sharedPath := filepath.Join(dir, "shared", "finish.json")
	localPath := filepath.Join(dir, "local", "finish.pending.json")

	m := newManager(sharedPath, localPath, "regatta-a")
	t.Cleanup(m.Close)

	if _, ok := m.Recovered(); ok {
		t.Error("expected nothing to recover when no local file exists")
	}
	if got := m.Status().State; got != StateSynced {
		t.Errorf("expected Synced, got %s", got)
	}
}

func TestRecoverPending_UnreadableLocalFile_LeftInPlace(t *testing.T) {
	dir := t.TempDir()
	sharedPath := filepath.Join(dir, "shared", "finish.json")
	localPath := filepath.Join(dir, "local", "finish.pending.json")

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		t.Fatalf("could not create local dir: %v", err)
	}
	if err := os.WriteFile(localPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("could not seed corrupt local file: %v", err)
	}

	m := newManager(sharedPath, localPath, "regatta-a")
	t.Cleanup(m.Close)

	if _, ok := m.Recovered(); ok {
		t.Error("expected an unparseable local entry not to be recovered")
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Errorf("expected the unreadable entry to be left in place, stat err = %v", err)
	}
}
