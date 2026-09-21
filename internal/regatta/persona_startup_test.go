package regatta

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// findInCanvasTree depth-first searches o for the first object match accepts,
// including inside a dialog's overlay: Fyne wraps overlay content in an
// unexported OverlayContainer, so anything that isn't one of the concrete
// container types below falls through to its renderer's objects (the
// sanctioned way to reach into an opaque fyne.Widget from a test).
func findInCanvasTree(o fyne.CanvasObject, match func(fyne.CanvasObject) bool) fyne.CanvasObject {
	if o == nil {
		return nil
	}
	if match(o) {
		return o
	}
	switch v := o.(type) {
	case *widget.PopUp:
		return findInCanvasTree(v.Content, match)
	case *container.AppTabs:
		for _, item := range v.Items {
			if got := findInCanvasTree(item.Content, match); got != nil {
				return got
			}
		}
	case *fyne.Container:
		for _, c := range v.Objects {
			if got := findInCanvasTree(c, match); got != nil {
				return got
			}
		}
	default:
		if w, ok := o.(fyne.Widget); ok {
			for _, c := range w.CreateRenderer().Objects() {
				if got := findInCanvasTree(c, match); got != nil {
					return got
				}
			}
		}
	}
	return nil
}

// findOverlayButtonByLabel returns the first button with the given text inside
// any of c's overlays (e.g. a dialog.CustomDialog's SetButtons row), or nil.
func findOverlayButtonByLabel(c fyne.Canvas, label string) *widget.Button {
	for _, ov := range c.Overlays().List() {
		found := findInCanvasTree(ov, func(o fyne.CanvasObject) bool {
			b, ok := o.(*widget.Button)
			return ok && b.Text == label
		})
		if b, ok := found.(*widget.Button); ok {
			return b
		}
	}
	return nil
}

func findAppTabs(o fyne.CanvasObject) *container.AppTabs {
	switch v := o.(type) {
	case *container.AppTabs:
		return v
	case *fyne.Container:
		for _, child := range v.Objects {
			if got := findAppTabs(child); got != nil {
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
			stopWithTimeout(t, r.stopWatcher)
		}
	})
}

// stopWithTimeout runs stop() with a deadline so a wedged watcher shutdown fails
// one test in seconds instead of hanging until go test's 10-minute package
// timeout. See docs/features/testing/known-issues.md.
func stopWithTimeout(t *testing.T, stop func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Error("watcher shutdown did not complete within 10s")
	}
}

// quiesceWatcher stops r's watcher (and its consumer goroutine) now, so a
// background watch event cannot drive a Fyne render concurrently with the test
// goroutine - Fyne's test driver runs fyne.Do inline and its global text shaper
// is not goroutine-safe (see docs/features/testing/known-issues.md). Use it in
// any test that starts a director/timer with PrefRegattaDir set and then drives
// a foreground render (showRaceTree / refreshContent / refreshAllRows /
// applyPendingOrigin / a schedule-guard proceed()).
func quiesceWatcher(t *testing.T, r *Regatta) {
	t.Helper()
	if r.stopWatcher != nil {
		stopWithTimeout(t, r.stopWatcher)
		r.stopWatcher = nil
	}
}

func TestTimerShowsPersonaPicker(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewTimer(app)

	tabs := findAppTabs(r.window.Content())
	if tabs == nil {
		t.Fatal("persona picker has no tabs")
	}
	gotTabs := make([]string, len(tabs.Items))
	for i, it := range tabs.Items {
		gotTabs[i] = it.Text
	}
	wantTabs := []string{common.PersonaTabTimers, common.PersonaTabMedia, common.PersonaTabAdmins}
	if !slices.Equal(gotTabs, wantTabs) {
		t.Fatalf("tabs = %v, want %v", gotTabs, wantTabs)
	}

	labels := buttonLabels(r.window.Content())
	for _, want := range []string{
		"Primary Start Timer", "Primary Finish Timer", "Secondary Start Timer", "Secondary Finish Timer",
		persona.DirectorDefinition.Label,
		common.PersonaSocialMediaLabel, common.PersonaStreamingLabel,
		common.PersonaRegisterResultsLabel, common.PersonaDeveloperLabel,
	} {
		if !slices.Contains(labels, want) {
			t.Errorf("picker %v is missing %q", labels, want)
		}
	}

	if r.session.Root != "" {
		t.Error("session should not be bound before the picker completes")
	}
}

func TestPersonaPicker_PlaceholdersDisabled(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewTimer(app)
	content := r.window.Content()

	for _, label := range []string{
		common.PersonaSocialMediaLabel, common.PersonaStreamingLabel,
		common.PersonaRegisterResultsLabel, common.PersonaDeveloperLabel,
	} {
		b := findButtonByLabel(content, label)
		if b == nil {
			t.Errorf("placeholder %q not on the picker", label)
			continue
		}
		if !b.Disabled() {
			t.Errorf("placeholder %q should be disabled", label)
		}
		if b.OnTapped != nil {
			t.Errorf("placeholder %q should have no action", label)
		}
	}

	for _, label := range []string{
		"Primary Start Timer", "Primary Finish Timer",
		"Secondary Start Timer", "Secondary Finish Timer",
		persona.DirectorDefinition.Label,
	} {
		b := findButtonByLabel(content, label)
		if b == nil || b.Disabled() {
			t.Errorf("real persona %q should be an enabled button (got %v)", label, b)
		}
	}

	if r.session.Root != "" {
		t.Error("no session before a real persona is pressed")
	}
}

func TestPersonaPicker_ButtonOpensChallengePrompt(t *testing.T) {
	app := test.NewTempApp(t)
	r := New(app)

	btn := findButtonByLabel(r.window.Content(), "Primary Finish Timer")
	if btn == nil {
		t.Fatal("no Primary Finish Timer button on the picker")
	}
	btn.OnTapped()

	// Pressing a persona opens a challenge prompt - it must not itself be a
	// "wrong challenge" error, and no session/flow starts yet.
	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Error("pressing a persona button should open the challenge prompt")
	}
	if r.session.Root != "" || r.mode != modeUnset {
		t.Errorf("no session/mode until the challenge is entered: root=%q mode=%v", r.session.Root, r.mode)
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
	if findAppTabs(r.window.Content()) != nil {
		t.Error("still on the persona picker after starting the session")
	}
}

func TestStartSessionSetsRoleLabels(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if r.persona.Text != "Primary Start Timer" {
		t.Errorf("header role line = %q, want %q", r.persona.Text, "Primary Start Timer")
	}
	if got := r.window.Title(); got != "Regatta Clock — Primary Start Timer" {
		t.Errorf("window title = %q, want %q", got, "Regatta Clock — Primary Start Timer")
	}
	if _, texts := countObjects(r.treeTitle()); texts != 8 {
		t.Errorf("tree title text objects = %d, want 8 (role, name, subtitle, date, each with its own key)", texts)
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

func TestStartSessionPrimaryFinishMirrorsSecondaryFinish(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	sft := timerSession(t, "sft", root)
	secLog := &store.FinishLog{Races: map[int]store.RaceResult{
		2: {RaceNumber: 2, WinningTime: "07:00.0"},
	}}
	secLog.RegattaKey = key
	if err := store.SaveFinish(sft, secLog); err != nil {
		t.Fatal(err)
	}

	// A primary finish timer mirrors the secondary team's finish.json read-only.
	pft := timerSession(t, "pft", root)
	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	if r.secondaryFinishPath == "" {
		t.Fatal("primary FT should record the secondary finish path")
	}
	if r.secondaryFinishLog == nil || r.secondaryFinishLog.Races[2].WinningTime != "07:00.0" {
		t.Fatalf("secondary finish.json not mirrored: %+v", r.secondaryFinishLog)
	}

	// A secondary finish timer does not mirror anything.
	r2 := NewTimer(app)
	stopWatch(t, r2)
	r2.startSession(timerSession(t, "sft", root), sch)
	if r2.secondaryFinishLog != nil || r2.secondaryFinishPath != "" {
		t.Errorf("the secondary FT must not mirror a secondary log: log=%v path=%q",
			r2.secondaryFinishLog, r2.secondaryFinishPath)
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

func TestStartSession_PersistsLastRegattaRoot(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if got := app.Preferences().String(common.PrefLastRegattaRoot); got != root {
		t.Errorf("PrefLastRegattaRoot = %q, want %q", got, root)
	}
}

func TestOnPersonaChosen_NoLastRoot_OpensFolderBrowserDirectly(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewTimer(app)
	stopWatch(t, r)

	def, _ := persona.ByID("pst")
	r.onPersonaChosen(def, "rc-pst")

	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Fatal("choosing a persona with no last root should still open the folder browser")
	}
	if findOverlayButtonByLabel(r.window.Canvas(), common.LoadPreviousRegattaButtonText) != nil {
		t.Error("no previous-regatta dialog should appear when PrefLastRegattaRoot is unset")
	}
	if r.session.Root != common.EmptyString {
		t.Error("no session should be bound yet")
	}
}

func TestOnPersonaChosen_UnreadableLastRoot_FallsBackToFolderBrowser(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefLastRegattaRoot, filepath.Join(t.TempDir(), "gone"))
	r := NewTimer(app)
	stopWatch(t, r)

	def, _ := persona.ByID("pst")
	r.onPersonaChosen(def, "rc-pst")

	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Fatal("an unreadable last root should still fall back to the folder browser")
	}
	if findOverlayButtonByLabel(r.window.Canvas(), common.LoadPreviousRegattaButtonText) != nil {
		t.Error("no previous-regatta dialog should appear for an unreadable root")
	}
}

func TestOnPersonaChosen_ValidLastRoot_OffersPreviousRegatta(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefLastRegattaRoot, root)
	r := NewTimer(app)
	stopWatch(t, r)

	def, _ := persona.ByID("pst")
	r.onPersonaChosen(def, "rc-pst")

	if findOverlayButtonByLabel(r.window.Canvas(), common.LoadPreviousRegattaButtonText) == nil {
		t.Fatal("a readable last root should offer to load the previous regatta")
	}
	if findOverlayButtonByLabel(r.window.Canvas(), common.ChooseAnotherFolderButtonText) == nil {
		t.Error("the previous-regatta dialog is missing its \"choose another folder\" button")
	}
	if findOverlayButtonByLabel(r.window.Canvas(), common.CancelButtonText) == nil {
		t.Error("the previous-regatta dialog is missing its cancel button")
	}
	if r.session.Root != common.EmptyString {
		t.Error("the session should not bind until the dialog is accepted")
	}
}

func TestConfirmPreviousRegatta_Yes_StartsSession(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefLastRegattaRoot, root)
	r := NewTimer(app)
	stopWatch(t, r)

	def, _ := persona.ByID("pst")
	r.onPersonaChosen(def, "rc-pst")

	yes := findOverlayButtonByLabel(r.window.Canvas(), common.LoadPreviousRegattaButtonText)
	if yes == nil {
		t.Fatal("no \"load previous regatta\" button found")
	}
	yes.OnTapped()

	if r.session.Root != root || r.session.Role != persona.RoleStart {
		t.Fatalf("session = %+v, want root %q", r.session, root)
	}
	if findAppTabs(r.window.Content()) != nil {
		t.Error("still on the persona picker after loading the previous regatta")
	}
}

func TestConfirmPreviousRegatta_No_OpensFolderBrowser(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefLastRegattaRoot, root)
	r := NewTimer(app)
	stopWatch(t, r)

	def, _ := persona.ByID("pst")
	r.onPersonaChosen(def, "rc-pst")

	no := findOverlayButtonByLabel(r.window.Canvas(), common.ChooseAnotherFolderButtonText)
	if no == nil {
		t.Fatal("no \"choose a different folder\" button found")
	}
	no.OnTapped()

	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Fatal("declining the previous regatta should open the folder browser")
	}
	if r.session.Root != common.EmptyString {
		t.Error("no session should be bound yet")
	}
}

func TestConfirmPreviousRegatta_Cancel_ReturnsToPicker(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefLastRegattaRoot, root)
	r := NewTimer(app)
	stopWatch(t, r)

	def, _ := persona.ByID("pst")
	r.onPersonaChosen(def, "rc-pst")

	cancel := findOverlayButtonByLabel(r.window.Canvas(), common.CancelButtonText)
	if cancel == nil {
		t.Fatal("no cancel button found")
	}
	cancel.OnTapped()

	if findAppTabs(r.window.Content()) == nil {
		t.Error("cancelling the previous-regatta dialog should return to the persona picker")
	}
	if r.session.Root != common.EmptyString {
		t.Error("no session should be bound after cancelling")
	}
}
