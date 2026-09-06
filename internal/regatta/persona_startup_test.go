package regatta

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

func findRadioGroup(o fyne.CanvasObject) *widget.RadioGroup {
	switch v := o.(type) {
	case *widget.RadioGroup:
		return v
	case *fyne.Container:
		for _, child := range v.Objects {
			if got := findRadioGroup(child); got != nil {
				return got
			}
		}
	}
	return nil
}

func menuLabels(m *fyne.MainMenu) []string {
	var out []string
	for _, menu := range m.Items {
		for _, item := range menu.Items {
			out = append(out, item.Label)
		}
	}
	return out
}

func timerSession(t *testing.T, id, root string) persona.Session {
	t.Helper()
	def, ok := persona.ByID(id)
	if !ok {
		t.Fatalf("unknown persona %q", id)
	}
	return persona.Session{Definition: def, Root: root}
}

func testSchedule() *store.Schedule {
	return &store.Schedule{
		Name: "Head of the Test",
		Date: "2026-10-01",
		Races: []store.ScheduleRace{
			{RaceNumber: 1, BoatClass: "V8", BoatCount: 2, Lanes: map[int]store.ScheduleEntry{
				1: {SchoolName: "Alpha"}, 2: {SchoolName: "Beta"},
			}},
			{RaceNumber: 2, Lanes: map[int]store.ScheduleEntry{}},
		},
	}
}

// seedRegatta writes a schedule into a fresh <tmp>/regattaData and returns the
// root path.
func seedRegatta(t *testing.T, sch *store.Schedule) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), common.RegattaDataDir)
	dir := timerSession(t, "rd", root)
	if err := store.SaveSchedule(dir, sch); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	return root
}

func stopWatch(t *testing.T, r *Regatta) {
	t.Cleanup(func() {
		if r.stopWatcher != nil {
			r.stopWatcher()
		}
	})
}

func TestTimerShowsPersonaPicker(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewTimer(app)

	rg := findRadioGroup(r.window.Content())
	if rg == nil {
		t.Fatal("persona picker has no radio group")
	}
	want := make([]string, len(persona.Registry))
	for i, d := range persona.Registry {
		want[i] = d.Label
	}
	if len(rg.Options) != len(want) {
		t.Fatalf("picker options = %v, want %v", rg.Options, want)
	}
	if r.session.Root != "" {
		t.Error("session should not be bound before the picker completes")
	}
}

func TestPersonaByLabel(t *testing.T) {
	if d, ok := personaByLabel("Primary Start Timer"); !ok || d.ID != "pst" {
		t.Errorf("personaByLabel(Primary Start Timer) = %+v, %v", d, ok)
	}
	if _, ok := personaByLabel("Nobody"); ok {
		t.Error("personaByLabel(Nobody) reported ok")
	}
}

func TestResolvePersonaRoot(t *testing.T) {
	// Picking regattaData itself.
	named := filepath.Join(t.TempDir(), common.RegattaDataDir)
	if err := os.MkdirAll(named, 0755); err != nil {
		t.Fatal(err)
	}
	if got := resolvePersonaRoot(named); got != named {
		t.Errorf("regattaData itself: got %q, want %q", got, named)
	}

	// Picking the parent folder that contains a populated regattaData - the
	// case operators reach for.
	root := seedRegatta(t, testSchedule()) // <tmp>/regattaData with a schedule
	parent := filepath.Dir(root)
	if got := resolvePersonaRoot(parent); got != root {
		t.Errorf("parent of regattaData: got %q, want %q", got, root)
	}

	// Picking a renamed shortcut that holds the schedule directly.
	renamed := filepath.Join(t.TempDir(), "Shared Regatta")
	if err := os.MkdirAll(filepath.Join(renamed, "director"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(persona.SchedulePathIn(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(persona.SchedulePathIn(renamed), data, 0644); err != nil {
		t.Fatal(err)
	}
	if got := resolvePersonaRoot(renamed); got != renamed {
		t.Errorf("renamed folder with a schedule: got %q, want %q", got, renamed)
	}

	// An unrelated folder with no regattaData child.
	if got := resolvePersonaRoot(t.TempDir()); got != "" {
		t.Errorf("unrelated folder: got %q, want \"\"", got)
	}
}

func TestScheduledRaceCount(t *testing.T) {
	if got := scheduledRaceCount(testSchedule()); got != 1 {
		t.Fatalf("scheduledRaceCount = %d, want 1 (race 2 has no lanes)", got)
	}
}

func TestStartSessionStartTimerHydratesOwnStart(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	log := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, Display: "09:00:00.0"},
	}}
	log.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveStart(pst, log); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if r.session.Role != persona.RoleStart || r.session.Root != root {
		t.Fatalf("session = %+v", r.session)
	}
	if r.writesBlocked {
		t.Error("writes should not be blocked for a clean file")
	}
	if r.startLog == nil || r.startLog.Races[1].Display != "09:00:00.0" {
		t.Fatalf("start log not restored: %+v", r.startLog)
	}
	if len(r.RegattaData.Races) != 2 {
		t.Fatalf("schedule not hydrated: %d races", len(r.RegattaData.Races))
	}
	if findRadioGroup(r.window.Content()) != nil {
		t.Error("still on the persona picker after starting the session")
	}
}

func TestStartSessionRejectsDifferentRegatta(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	log := &store.StartLog{Races: map[int]store.StartRecord{9: {RaceNumber: 9, Display: "stale"}}}
	log.RegattaKey = "not-this-regatta"
	if err := store.SaveStart(pst, log); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if len(r.startLog.Races) != 0 {
		t.Fatalf("stale races should not be shown: %+v", r.startLog.Races)
	}
	if _, err := os.Stat(pst.StartPath()); !os.IsNotExist(err) {
		t.Error("the different-regatta start.json should have been moved aside")
	}
	aside, _ := filepath.Glob(pst.StartPath() + ".other-regatta-*")
	if len(aside) != 1 {
		t.Fatalf("expected one set-aside file, found %v", aside)
	}
}

func TestStartSessionCorruptOwnFileBlocksWrites(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	if err := os.MkdirAll(filepath.Dir(pst.StartPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pst.StartPath(), []byte("{ not json"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if !r.writesBlocked {
		t.Fatal("a corrupt own file must block writes")
	}
	if r.startLog == nil || r.startLog.Races == nil {
		t.Fatalf("start log should be an empty non-nil map, got %+v", r.startLog)
	}
	if _, err := os.Stat(pst.StartPath()); err != nil {
		t.Error("the original corrupt file should be left in place")
	}
	aside, _ := filepath.Glob(pst.StartPath() + ".corrupt-*")
	if len(aside) != 1 {
		t.Fatalf("expected one .corrupt- copy, found %v", aside)
	}
}

func TestStartSessionFinishTimerHydratesPeerAndOwn(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	pst := timerSession(t, "pst", root)
	startLog := &store.StartLog{Races: map[int]store.StartRecord{1: {RaceNumber: 1, Display: "peer-start"}}}
	startLog.RegattaKey = key
	if err := store.SaveStart(pst, startLog); err != nil {
		t.Fatal(err)
	}

	pft := timerSession(t, "pft", root)
	now := time.Now().UTC()
	finishLog := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "06:00.0", UpdatedAt: now},
	}}
	finishLog.RegattaKey = key
	if err := store.SaveFinish(pft, finishLog); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	if r.writesBlocked {
		t.Error("clean files should not block writes")
	}
	if r.startLog == nil || r.startLog.Races[1].Display != "peer-start" {
		t.Fatalf("peer start not hydrated: %+v", r.startLog)
	}
	if r.finishLog == nil || r.finishLog.Races[1].WinningTime != "06:00.0" {
		t.Fatalf("own finish not hydrated: %+v", r.finishLog)
	}
}

func TestStartSessionFinishTimerPeerCorruptDoesNotBlock(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pft := timerSession(t, "pft", root)

	peer := timerSession(t, "pst", root).StartPath()
	if err := os.MkdirAll(filepath.Dir(peer), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(peer, []byte("garbage"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	if r.writesBlocked {
		t.Error("a corrupt PEER file must not block the finish timer's own writes")
	}
	if len(r.startLog.Races) != 0 {
		t.Fatalf("corrupt peer file should yield no start times: %+v", r.startLog.Races)
	}
	if _, err := os.Stat(peer); err != nil {
		t.Error("the finish timer must not rename the peer's file")
	}
}

func TestDirectorMenuHasImportTimerDoesNot(t *testing.T) {
	app := test.NewTempApp(t)

	dir := NewDirector(app)
	if !slices.Contains(menuLabels(dir.window.MainMenu()), common.LoadDataTitle) {
		t.Error("director menu should offer Excel import")
	}

	tmr := NewTimer(app)
	stopWatch(t, tmr)
	if slices.Contains(menuLabels(tmr.window.MainMenu()), common.LoadDataTitle) {
		t.Error("timer menu must not offer Excel import")
	}
}
