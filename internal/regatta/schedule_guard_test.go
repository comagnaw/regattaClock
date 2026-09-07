package regatta

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// directorAt binds a director to filepath.Dir(root) as its PrefRegattaDir and
// returns it with the watcher stopped.
func directorAt(t *testing.T, root string) *Regatta {
	t.Helper()
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))
	r := NewDirector(app)
	stopWatch(t, r)
	return r
}

func TestClassifyScheduleWrite_Fresh(t *testing.T) {
	root := filepath.Join(t.TempDir(), common.RegattaDataDir)
	r := directorAt(t, root)
	r.RegattaData = &reader.RegattaData{Name: "New Regatta", Date: "2026-01-01"}

	got, existing := r.classifyScheduleWrite(persona.Session{Definition: persona.DirectorDefinition, Root: root})
	if got != writeFresh || existing != nil {
		t.Errorf("classify = %d / %+v, want writeFresh / nil", got, existing)
	}
}

func TestClassifyScheduleWrite_SameRegatta(t *testing.T) {
	sch := testSchedule()
	root := seedRegatta(t, sch)
	r := directorAt(t, root)
	// NewDirector loaded the schedule, so r.RegattaData is that regatta.

	got, _ := r.classifyScheduleWrite(persona.Session{Definition: persona.DirectorDefinition, Root: root})
	if got != writeSameRegatta {
		t.Errorf("classify = %d, want writeSameRegatta", got)
	}
}

func TestClassifyScheduleWrite_DifferentRegatta_NoTiming(t *testing.T) {
	root := seedRegatta(t, testSchedule())
	r := directorAt(t, root)
	r.RegattaData = &reader.RegattaData{Name: "A Different Event", Date: "2027-05-05"}

	got, existing := r.classifyScheduleWrite(persona.Session{Definition: persona.DirectorDefinition, Root: root})
	if got != replaceDifferentRegatta {
		t.Fatalf("classify = %d, want replaceDifferentRegatta", got)
	}
	if existing.Name != testSchedule().Name {
		t.Errorf("existing schedule = %q, want the seeded one", existing.Name)
	}
}

func TestClassifyScheduleWrite_DifferentRegatta_WithTiming(t *testing.T) {
	sch := testSchedule()
	root := seedRegatta(t, sch)

	// A primary start timer has already written here.
	pst := timerSession(t, "pst", root)
	sl := &store.StartLog{Races: map[int]store.StartRecord{1: {RaceNumber: 1}}}
	sl.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveStart(pst, sl); err != nil {
		t.Fatal(err)
	}

	r := directorAt(t, root)
	r.RegattaData = &reader.RegattaData{Name: "A Different Event", Date: "2027-05-05"}

	got, _ := r.classifyScheduleWrite(persona.Session{Definition: persona.DirectorDefinition, Root: root})
	if got != blockDifferentRegatta {
		t.Errorf("classify = %d, want blockDifferentRegatta (timing data present)", got)
	}
}

func TestClassifyScheduleWrite_Unreadable(t *testing.T) {
	root := filepath.Join(t.TempDir(), common.RegattaDataDir)
	dir := persona.Session{Definition: persona.DirectorDefinition, Root: root}
	if err := os.MkdirAll(filepath.Dir(dir.SchedulePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir.SchedulePath(), []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := directorAt(t, root)
	r.RegattaData = &reader.RegattaData{Name: "Anything", Date: "2026-01-01"}

	got, _ := r.classifyScheduleWrite(dir)
	if got != blockUnreadable {
		t.Errorf("classify = %d, want blockUnreadable", got)
	}
}

func TestSetAsideScheduleThenReplace(t *testing.T) {
	sch := testSchedule()
	root := seedRegatta(t, sch)
	oldKey := store.RegattaKey(sch.Name, sch.Date)
	dir := persona.Session{Definition: persona.DirectorDefinition, Root: root}

	r := directorAt(t, root)
	r.RegattaData = &reader.RegattaData{Name: "Fresh Event", Date: "2028-08-08"}

	r.setAsideSchedule(dir, oldKey)
	r.saveRegattaData()

	aside := filepath.Join(filepath.Dir(dir.SchedulePath()), "regattaSchedule."+oldKey+".json")
	if _, err := os.Stat(aside); err != nil {
		t.Errorf("previous schedule not kept aside at %s: %v", aside, err)
	}
	now, err := store.LoadSchedule(dir)
	if err != nil {
		t.Fatalf("LoadSchedule after replace: %v", err)
	}
	if now.Name != "Fresh Event" {
		t.Errorf("regattaSchedule.json = %q, want the new regatta", now.Name)
	}
}

func TestGuardScheduleWrite_SameRegattaProceeds(t *testing.T) {
	root := seedRegatta(t, testSchedule())
	r := directorAt(t, root)

	proceeded := false
	r.guardScheduleWrite(func() { proceeded = true })
	if !proceeded {
		t.Error("a same-regatta write should proceed without a prompt")
	}
}

func TestGuardScheduleWrite_BlockedDoesNotProceed(t *testing.T) {
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)
	sl := &store.StartLog{Races: map[int]store.StartRecord{1: {RaceNumber: 1}}}
	sl.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveStart(pst, sl); err != nil {
		t.Fatal(err)
	}

	r := directorAt(t, root)
	r.RegattaData = &reader.RegattaData{Name: "Different", Date: "2027-01-01"}

	proceeded := false
	r.guardScheduleWrite(func() { proceeded = true })
	if proceeded {
		t.Error("a blocked different-regatta import must not proceed")
	}
	// The on-disk schedule is untouched.
	now, _ := store.LoadSchedule(persona.Session{Definition: persona.DirectorDefinition, Root: root})
	if now.Name != sch.Name {
		t.Errorf("schedule overwritten despite the block: %q", now.Name)
	}
}

func TestReloadScheduleNoOrigin(t *testing.T) {
	root := seedRegatta(t, testSchedule()) // Origin.URI is empty
	r := directorAt(t, root)
	// Must not panic; just informs the operator.
	r.reloadSchedule()
}

func TestDirectorMenuHasReloadSchedule(t *testing.T) {
	app := test.NewTempApp(t)
	dir := NewDirector(app)
	if !slices.Contains(menuLabels(dir.window.MainMenu()), common.ReloadScheduleTitle) {
		t.Error("director menu is missing Reload Schedule")
	}

	tmr := NewTimer(app)
	if slices.Contains(menuLabels(tmr.window.MainMenu()), common.ReloadScheduleTitle) {
		t.Error("a timer must not have Reload Schedule")
	}
}
