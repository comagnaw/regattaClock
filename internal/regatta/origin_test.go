package regatta

import (
	"path/filepath"
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
	xlsx, err := filepath.Abs(filepath.Join("..", "..", "examples", "Example Regatta Input Table.xlsx"))
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

func TestReloadScheduleUnchangedDoesNotRewrite(t *testing.T) {
	xlsx, err := filepath.Abs(filepath.Join("..", "..", "examples", "Example Regatta Input Table.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	rd, err := reader.ReadExcelFile(xlsx)
	if err != nil {
		t.Fatal(err)
	}

	// Seed the schedule from that same workbook, so a reload is a no-op. A
	// sentinel Origin.Hash proves whether SaveSchedule ran.
	sch := scheduleFromRegattaData(rd)
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
