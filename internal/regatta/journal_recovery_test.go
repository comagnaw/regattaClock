package regatta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// seedLocalJournalEntry writes v as the local write-ahead journal entry
// journal.For would compute for a session rooted at root, simulating a value
// durably captured on a previous run that crashed before it ever reached the
// shared path. It reproduces the journal package's own local-path formula (a
// hash of Session.Root, then team, then role) rather than going through
// journal.For, because that call's crash-recovery check only ever runs once
// per path per process - using it to seed here would consume the one chance
// meant for hydrateOwnStart/hydrateOwnFinish's own later call.
func seedLocalJournalEntry(t *testing.T, root string, team persona.Team, role persona.Role, v any) {
	t.Helper()
	dir := filepath.Join(journalTestRoot, filesystem.HashBytes([]byte(root))[:16], string(team))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("could not create local journal dir: %v", err)
	}
	mustWriteJSONFile(t, filepath.Join(dir, string(role)+".pending.json"), v)
}

// mustWriteSharedFile writes v directly to path, exactly as it would look on
// disk after a real save, but without going through store.SaveStart/SaveFinish
// - which would call journal.For itself and either consume its one-time
// crash-recovery check before hydrateOwnStart/hydrateOwnFinish ever runs, or
// overwrite a value seeded by seedLocalJournalEntry above. hydrateOwnStart's
// own later call must be the first thing in the test process that constructs
// a journal.Manager for this session's path.
func mustWriteSharedFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("could not create dir for %s: %v", path, err)
	}
	mustWriteJSONFile(t, path, v)
}

func mustWriteJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("could not marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("could not write %s: %v", path, err)
	}
}

func TestStartSessionRecoversUnflushedStartWrite(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)
	key := store.RegattaKey(sch.Name, sch.Date)

	// The shared file as it was left after the crash: race 1 only, at
	// Sequence 1. Written directly, not via store.SaveStart - that would
	// itself construct the journal.Manager for this path (the first call in
	// this test process wins the one-time crash-recovery check), and it must
	// be hydrateOwnStart, later, that does that.
	shared := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, Display: "09:00:00.0"},
	}}
	shared.RegattaKey = key
	shared.Sequence = 1
	mustWriteSharedFile(t, pst.StartPath(), shared)

	// The local journal entry that never made it to the shared file before
	// the crash: race 2 added, at a newer Sequence.
	local := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, Display: "09:00:00.0"},
		2: {RaceNumber: 2, Display: "09:05:00.0"},
	}}
	local.RegattaKey = key
	local.Sequence = 2
	seedLocalJournalEntry(t, root, persona.TeamPrimary, persona.RoleStart, local)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if got := len(r.startLog.Races); got != 2 {
		t.Fatalf("expected the recovered 2-race map to win over the 1-race shared file, got %d races: %+v",
			got, r.startLog.Races)
	}
	if r.startLog.Races[2].Display != "09:05:00.0" {
		t.Errorf("recovered race 2 missing or wrong: %+v", r.startLog.Races[2])
	}
}

func TestStartSessionRecoversUnflushedFinishWrite(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pft := timerSession(t, "pft", root)
	key := store.RegattaKey(sch.Name, sch.Date)

	shared := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "5:00.0"},
	}}
	shared.RegattaKey = key
	shared.Sequence = 1
	mustWriteSharedFile(t, pft.FinishPath(), shared)

	local := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "5:00.0"},
		2: {RaceNumber: 2, WinningTime: "5:10.0"},
	}}
	local.RegattaKey = key
	local.Sequence = 2
	seedLocalJournalEntry(t, root, persona.TeamPrimary, persona.RoleFinish, local)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	if got := len(r.finishLog.Races); got != 2 {
		t.Fatalf("expected the recovered 2-race map to win over the 1-race shared file, got %d races: %+v",
			got, r.finishLog.Races)
	}
	if r.finishLog.Races[2].WinningTime != "5:10.0" {
		t.Errorf("recovered race 2 missing or wrong: %+v", r.finishLog.Races[2])
	}
}

// TestStartSessionCorruptOwnFileSkipsRecovery locks in a deliberate scope
// decision: a corrupt shared file already demands the operator's attention
// (blockWritesForCorruptFile), and silently substituting a recovered value
// there would only obscure that. hydrateOwnStart's corrupt-file branch must
// keep returning empty even when a recovered entry exists.
func TestStartSessionCorruptOwnFileSkipsRecovery(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)
	key := store.RegattaKey(sch.Name, sch.Date)

	if err := os.MkdirAll(filepath.Dir(pst.StartPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pst.StartPath(), []byte("{ not json"), 0644); err != nil {
		t.Fatal(err)
	}

	local := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, Display: "09:00:00.0"},
	}}
	local.RegattaKey = key
	local.Sequence = 1
	seedLocalJournalEntry(t, root, persona.TeamPrimary, persona.RoleStart, local)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if !r.writesBlocked {
		t.Fatal("a corrupt own file must still block writes even when a recovered entry exists")
	}
	if len(r.startLog.Races) != 0 {
		t.Fatalf("expected the corrupt-file branch to skip recovery and stay empty, got %+v", r.startLog.Races)
	}
}
