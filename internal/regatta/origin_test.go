package regatta

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func rdWith(name, date string, races ...reader.RaceData) *reader.RegattaData {
	return &reader.RegattaData{Name: name, Date: date, Races: races}
}

func oneRace(n int, school string) reader.RaceData {
	return reader.RaceData{RaceNumber: n, BoatCount: 2, Lanes: map[int]reader.RaceEntry{1: {SchoolName: school}}}
}

// directorWithBanner builds a director with its header banners realised and a
// live RegattaData set, ready for origin-inspection.
func directorWithBanner(t *testing.T, live *reader.RegattaData) *Regatta {
	t.Helper()
	r := directorWithRaces(t, nil)
	r.RegattaData = live
	r.directorHeaderExtras() // creates r.originBanner
	return r
}

func TestInspectOriginCandidate_UnchangedContent(t *testing.T) {
	live := rdWith("Test", "2026-10-02", oneRace(1, "A"), oneRace(2, "B"))
	r := directorWithBanner(t, live)

	// Same content, different Origin metadata (an unrelated workbook save).
	cand := rdWith("Test", "2026-10-02", oneRace(1, "A"), oneRace(2, "B"))
	cand.SourceInfo = reader.SourceInfo{Hash: "new-bytes"}

	r.inspectOriginCandidate(cand)

	if !r.originBanner.root.Hidden {
		t.Error("banner should stay hidden when the schedule content is unchanged")
	}
	if r.pendingOrigin != nil {
		t.Error("no pending origin for an unchanged workbook")
	}
}

func TestInspectOriginCandidate_ChangedContent(t *testing.T) {
	r := directorWithBanner(t, rdWith("Test", "2026-10-02", oneRace(1, "A"), oneRace(2, "B")))

	cand := rdWith("Test", "2026-10-02", oneRace(1, "Moved"), oneRace(2, "B"), oneRace(3, "C"))
	r.inspectOriginCandidate(cand)

	if r.originBanner.root.Hidden {
		t.Fatal("banner should show for a real schedule change")
	}
	if r.pendingOrigin != cand {
		t.Error("the candidate should be held as pendingOrigin")
	}
	if txt := r.originBanner.label.Text; !strings.Contains(txt, "changed") && !strings.Contains(txt, "added") {
		t.Errorf("banner text = %q, want a change summary", txt)
	}
}

func TestDismissOriginSuppressesReNag(t *testing.T) {
	r := directorWithBanner(t, rdWith("Test", "2026-10-02", oneRace(1, "A")))
	cand := rdWith("Test", "2026-10-02", oneRace(1, "B"))

	r.inspectOriginCandidate(cand)
	if r.originBanner.root.Hidden {
		t.Fatal("precondition: banner shown")
	}

	r.dismissOrigin()
	if !r.originBanner.root.Hidden || r.pendingOrigin != nil {
		t.Fatal("dismiss should hide and clear pending")
	}

	// The poll re-reads the same unchanged workbook: no re-nag.
	r.inspectOriginCandidate(rdWith("Test", "2026-10-02", oneRace(1, "B")))
	if !r.originBanner.root.Hidden {
		t.Error("a dismissed change must not re-raise the banner for the same content")
	}

	// A further, different change does raise it again.
	r.inspectOriginCandidate(rdWith("Test", "2026-10-02", oneRace(1, "C")))
	if r.originBanner.root.Hidden {
		t.Error("a new change after a dismissal should raise the banner")
	}
}

func TestApplyPendingOrigin_SameRegattaWrites(t *testing.T) {
	sch := testSchedule()
	root := seedRegatta(t, sch)
	r := directorAt(t, root)
	r.directorHeaderExtras()

	// A changed candidate for the SAME regatta (name/date unchanged).
	cand := regattaDataFromSchedule(sch)
	cand.Races[0].Lanes[1] = reader.RaceEntry{SchoolName: "Relabelled Crew"}
	r.pendingOrigin = cand
	r.originBanner.show("x")

	r.applyPendingOrigin()

	if r.pendingOrigin != nil || !r.originBanner.root.Hidden {
		t.Error("apply should clear pending and hide the banner")
	}
	got, err := store.LoadSchedule(persona.Session{Definition: persona.DirectorDefinition, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if got.Races[0].Lanes[1].SchoolName != "Relabelled Crew" {
		t.Errorf("schedule on disk not updated: lane 1 = %q", got.Races[0].Lanes[1].SchoolName)
	}
}

func TestApplyPendingOrigin_DifferentRegattaWithTimingRefused(t *testing.T) {
	sch := testSchedule()
	root := seedRegatta(t, sch)

	pst := timerSession(t, "pst", root)
	sl := &store.StartLog{Races: map[int]store.StartRecord{1: {RaceNumber: 1}}}
	sl.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveStart(pst, sl); err != nil {
		t.Fatal(err)
	}

	r := directorAt(t, root)
	r.directorHeaderExtras()
	r.pendingOrigin = rdWith("A Different Event", "2027-05-05", oneRace(1, "Z"))

	r.applyPendingOrigin()

	got, _ := store.LoadSchedule(persona.Session{Definition: persona.DirectorDefinition, Root: root})
	if got.Name != sch.Name {
		t.Errorf("schedule overwritten despite the different-regatta block: %q", got.Name)
	}
}

func TestPollOriginNoChange(t *testing.T) {
	xlsx, err := filepath.Abs(filepath.Join("..", "..", "examples", "Example Heat Sheets and Results With Macros.xlsm"))
	if err != nil {
		t.Fatal(err)
	}
	rd, err := reader.ReadExcelFile(xlsx)
	if err != nil {
		t.Fatal(err)
	}

	r := directorWithRaces(t, nil)
	got := r.pollOrigin(xlsx, rd.Hash)
	if got != rd.Hash {
		t.Errorf("pollOrigin advanced the hash for an unchanged file: %q -> %q", rd.Hash, got)
	}
}

// swapOriginHash replaces the origin poll's hash seam for one test.
func swapOriginHash(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	orig := originHashFunc
	originHashFunc = fn
	t.Cleanup(func() { originHashFunc = orig })
}

// TestPollOrigin_LockedWorkbookRaisesBanner: the RD has the heat sheet open in
// Excel, so every poll fails to read it. That must be visible, not a silent
// Debug line, and must clear itself once Excel lets go.
func TestPollOrigin_LockedWorkbookRaisesBanner(t *testing.T) {
	r := directorWithBanner(t, rdWith("Test", "2026-10-02", oneRace(1, "A")))
	uri := filepath.Join("share", "Heat Sheet.xlsm")

	swapOriginHash(t, func(string) (string, error) {
		return "", fmt.Errorf("open: %w", fs.ErrPermission)
	})
	if got := r.pollOrigin(uri, "accepted"); got != "accepted" {
		t.Errorf("a locked poll advanced the hash to %q", got)
	}
	if r.originLocked.root.Hidden {
		t.Fatal("locked banner should show while the workbook cannot be read")
	}
	if txt := r.originLocked.label.Text; !strings.Contains(txt, "Heat Sheet.xlsm") {
		t.Errorf("banner text = %q, want the workbook name", txt)
	}

	// Excel closed: the next poll reads the file and the banner clears.
	swapOriginHash(t, func(string) (string, error) { return "accepted", nil })
	r.pollOrigin(uri, "accepted")
	if !r.originLocked.root.Hidden {
		t.Error("locked banner should clear once the workbook is readable again")
	}
}

// TestPollOrigin_DismissSilencesOneLockedSpell: Dismiss hides the banner for
// the rest of that locked spell, but a later spell raises it again.
func TestPollOrigin_DismissSilencesOneLockedSpell(t *testing.T) {
	r := directorWithBanner(t, rdWith("Test", "2026-10-02", oneRace(1, "A")))
	locked := func(string) (string, error) { return "", fmt.Errorf("open: %w", fs.ErrPermission) }
	readable := func(string) (string, error) { return "h", nil }

	swapOriginHash(t, locked)
	r.pollOrigin("w.xlsx", "h")
	r.originLocked.dismiss.OnTapped()
	r.pollOrigin("w.xlsx", "h")
	if !r.originLocked.root.Hidden {
		t.Fatal("a dismissed banner should stay hidden for the same locked spell")
	}

	swapOriginHash(t, readable)
	r.pollOrigin("w.xlsx", "h")
	swapOriginHash(t, locked)
	r.pollOrigin("w.xlsx", "h")
	if r.originLocked.root.Hidden {
		t.Error("a new locked spell after the file was readable should raise the banner again")
	}
}

// TestPollOrigin_OtherFailureStaysQuiet: a failure that is not a lock (a
// share briefly unreachable, a mid-save) is retried without the banner.
func TestPollOrigin_OtherFailureStaysQuiet(t *testing.T) {
	r := directorWithBanner(t, rdWith("Test", "2026-10-02", oneRace(1, "A")))
	swapOriginHash(t, func(string) (string, error) { return "", fs.ErrNotExist })
	r.pollOrigin("w.xlsx", "h")
	if !r.originLocked.root.Hidden {
		t.Error("a non-lock read failure should not raise the locked banner")
	}
}

// TestSetRegattaData_LockedWorkbookMessage: Import / Reload Schedule on a
// workbook another program holds gets a "close it in Excel" message instead
// of the raw OS error. An unreadable file stands in for Excel's lock.
func TestSetRegattaData_LockedWorkbookMessage(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permissions do not block reads here")
	}
	path := filepath.Join(t.TempDir(), "Heat Sheet.xlsx")
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}

	r := directorWithRaces(t, nil)
	err := r.setRegattaData(path)
	if err == nil || !strings.Contains(err.Error(), "open in another program") {
		t.Errorf("setRegattaData(locked) error = %v, want the workbook-locked message", err)
	}
}

func TestReloadScheduleUnchangedDoesNotRewrite(t *testing.T) {
	xlsx, err := filepath.Abs(filepath.Join("..", "..", "examples", "Example Heat Sheets and Results With Macros.xlsm"))
	if err != nil {
		t.Fatal(err)
	}
	rd, err := reader.ReadExcelFile(xlsx)
	if err != nil {
		t.Fatal(err)
	}

	// Seed the schedule from that same workbook, so a reload is a no-op. A
	// sentinel Origin.Hash proves whether SaveSchedule ran.
	sch := store.ScheduleFromRegattaData(rd)
	sch.Origin.Hash = "SENTINEL-not-overwritten"
	root := seedRegatta(t, sch)
	dir := persona.Session{Definition: persona.DirectorDefinition, Root: root}

	r := directorAt(t, root)
	r.RegattaData.URI = xlsx // the seeded schedule has no Origin.URI

	r.reloadSchedule()

	after, _ := store.LoadSchedule(dir)
	if after.Origin.Hash != "SENTINEL-not-overwritten" {
		t.Errorf("reload rewrote an unchanged schedule (Origin.Hash = %q)", after.Origin.Hash)
	}
}
