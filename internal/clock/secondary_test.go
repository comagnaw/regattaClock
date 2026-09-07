package clock

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

func sftSession(t *testing.T) persona.Session {
	t.Helper()
	def, ok := persona.ByID("sft")
	if !ok {
		t.Fatal("no sft persona")
	}
	return persona.Session{Definition: def, Root: filepath.Join(t.TempDir(), common.RegattaDataDir)}
}

func openSecondaryClock(t *testing.T, s persona.Session, log *store.FinishLog) *Clock {
	t.Helper()
	app := test.NewTempApp(t)
	clk := NewClock(app, createTestRegattaData(), createTestRaceData()).WithFinishLog(s, log)
	clk.OpenRaceClock()
	t.Cleanup(clk.closeWindow)
	return clk
}

func buttonLabels(o fyne.CanvasObject) []string {
	var out []string
	switch v := o.(type) {
	case *widget.Button:
		out = append(out, v.Text)
	case *fyne.Container:
		for _, c := range v.Objects {
			out = append(out, buttonLabels(c)...)
		}
	}
	return out
}

func TestSecondaryFinish_NoRefereeButton(t *testing.T) {
	sec := openSecondaryClock(t, sftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	labels := buttonLabels(sec.approvalPanel())
	if slices.Contains(labels, common.RefereeButtonText) {
		t.Errorf("secondary approval panel has the Referee button: %v", labels)
	}
	if !slices.Contains(labels, common.SaveAndCloseButtonText) {
		t.Errorf("secondary approval panel is missing Save and Close: %v", labels)
	}

	pri := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	priLabels := buttonLabels(pri.approvalPanel())
	if !slices.Contains(priLabels, common.RefereeButtonText) || !slices.Contains(priLabels, common.CloseButtonText) {
		t.Errorf("primary approval panel should have Referee Approval + Close: %v", priLabels)
	}
	if slices.Contains(priLabels, common.SaveButtonText) || slices.Contains(priLabels, common.SaveAndCloseButtonText) {
		t.Errorf("primary approval panel should have no Save button: %v", priLabels)
	}
}

func TestSecondaryFinish_WinningTimeEnablesSaveDirectly(t *testing.T) {
	sec := openSecondaryClock(t, sftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})

	if !sec.buttons.save.Disabled() {
		t.Fatal("Save should start disabled")
	}
	sec.winningTime.SetText("01:00.0")
	if sec.buttons.save.Disabled() {
		t.Error("a valid winning time should enable Save for the secondary FT (no approval step)")
	}
	if !sec.buttons.referee.Disabled() {
		t.Error("the secondary FT's referee button stays disabled")
	}

	sec.winningTime.SetText("")
	if !sec.buttons.save.Disabled() {
		t.Error("clearing the winning time should disable Save again")
	}
}

func TestSecondaryFinish_SaveWritesUnapproved(t *testing.T) {
	s := sftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	sec := openSecondaryClock(t, s, log)

	closed := false
	sec.AfterClose = func() { closed = true }

	sec.laps.setOOFLaneNum(0, "2")
	sec.laps.setPlace(0, "1")
	sec.laps.setSplit(0, "00:00.0")
	sec.winningTime.SetText("06:00.0")
	sec.buttons.save.OnTapped()

	if !closed {
		t.Error("Save and Close should close the clock window")
	}

	res := log.Races[1]
	if res.WinningTime != "06:00.0" {
		t.Fatalf("winning time = %q", res.WinningTime)
	}
	if res.Approved || res.ApprovedAt != nil {
		t.Errorf("secondary Save must write Approved=false / ApprovedAt=nil: %+v", res)
	}
	if len(res.Rows) != 6 {
		t.Fatalf("want 6 lap rows, got %d", len(res.Rows))
	}

	onDisk, err := store.LoadFinish(s)
	if err != nil {
		t.Fatalf("LoadFinish: %v", err)
	}
	if onDisk.Races[1].WinningTime != "06:00.0" || onDisk.Races[1].Approved {
		t.Errorf("disk result = %+v", onDisk.Races[1])
	}
}

func TestSecondaryFinish_RehydrateEnablesSaveOnWinningTime(t *testing.T) {
	s := sftSession(t)
	seed := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {
			RaceNumber: 1, WinningTime: "02:00.0", Approved: false,
			Rows: []store.LapRow{
				{Lane: 2, Place: "1", Split: "00:00.0", Time: "02:00.0"},
				{}, {}, {}, {}, {},
			},
		},
	}}
	if err := store.SaveFinish(s, seed); err != nil {
		t.Fatal(err)
	}
	restored, _ := store.LoadFinish(s)

	sec := openSecondaryClock(t, s, restored)

	if sec.buttons.save.Disabled() {
		t.Error("rehydrating a saved (unapproved) secondary race should enable Save")
	}
	if !strings.HasPrefix(sec.commitStatus.Text, "Saved ") {
		t.Errorf("commit status = %q, want a \"Saved …\" line", sec.commitStatus.Text)
	}
	if sec.winningTime.Text != "02:00.0" {
		t.Errorf("winning time = %q, want 02:00.0", sec.winningTime.Text)
	}
	if sec.laps.oofLaneNum(0) != "2" || sec.laps.place(0) != "1" {
		t.Errorf("lap row not restored: oof=%q place=%q", sec.laps.oofLaneNum(0), sec.laps.place(0))
	}
}
