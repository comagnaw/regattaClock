package clock

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

func findButton(o fyne.CanvasObject, label string) *widget.Button {
	switch v := o.(type) {
	case *widget.Button:
		if v.Text == label {
			return v
		}
	case *fyne.Container:
		for _, c := range v.Objects {
			if b := findButton(c, label); b != nil {
				return b
			}
		}
	case *container.ThemeOverride:
		return findButton(v.Content, label)
	case *container.Scroll:
		return findButton(v.Content, label)
	}
	return nil
}

func openReferee(t *testing.T, clk *Clock) {
	t.Helper()
	clk.refereeFunc()()
	if clk.refereeWindow == nil {
		t.Fatal("Referee Approval did not open a window")
	}
	t.Cleanup(func() {
		if clk.refereeWindow != nil {
			clk.refereeWindow.Close()
		}
	})
}

func TestRefereeApproval_OpensSeparateWindowWithButtons(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	openReferee(t, clk)

	if clk.refereeWindow == clk.window {
		t.Error("the approval window must be independent of the clock window")
	}
	if findButton(clk.refereeWindow.Content(), common.ApproveButtonText) == nil {
		t.Error("approval window is missing the Approve button")
	}
	if findButton(clk.refereeWindow.Content(), common.CancelButtonText) == nil {
		t.Error("approval window is missing the Cancel button")
	}
}

func TestRefereeApproval_NoStacking(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	openReferee(t, clk)
	first := clk.refereeWindow

	clk.refereeFunc()() // second tap
	if clk.refereeWindow != first {
		t.Error("a second Referee tap must not open a second window")
	}
}

func TestRefereeApproval_ApproveWritesAndCloses(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)

	clk.buttons.start.OnTapped()
	clk.laps.setOOFLaneNum(0, "3")
	clk.laps.setPlace(0, "1")
	clk.laps.setSplit(0, "00:05.0")
	clk.winningTime.SetText("01:00.0")

	openReferee(t, clk)
	findButton(clk.refereeWindow.Content(), common.ApproveButtonText).OnTapped()

	if clk.refereeWindow != nil {
		t.Error("Approve should close the approval window")
	}
	if res := log.Races[1]; !res.Approved || res.WinningTime != "01:00.0" {
		t.Fatalf("approve result = %+v", res)
	}
	onDisk, err := store.LoadFinish(s)
	if err != nil || !onDisk.Races[1].Approved {
		t.Errorf("disk result not approved: %+v (err %v)", onDisk.Races[1], err)
	}
}

func TestRefereeApproval_CancelDoesNotWrite(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)
	clk.buttons.start.OnTapped()
	clk.winningTime.SetText("01:00.0")

	openReferee(t, clk)
	findButton(clk.refereeWindow.Content(), common.CancelButtonText).OnTapped()

	if clk.refereeWindow != nil {
		t.Error("Cancel should close the approval window")
	}
	if log.Races[1].Approved {
		t.Error("Cancel must not approve the race")
	}
}

func TestRefereeApproval_BlocksAndRestoresClock(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	clk.winningTime.SetText("01:00.0") // makes the referee button eligible

	openReferee(t, clk)
	if !clk.buttons.start.Disabled() || !clk.winningTime.Disabled() || !clk.laps[0].oofLaneNum.Disabled() {
		t.Error("the clock controls should be disabled while the approval window is open")
	}

	findButton(clk.refereeWindow.Content(), common.CancelButtonText).OnTapped()
	if clk.buttons.start.Disabled() || clk.buttons.referee.Disabled() || clk.winningTime.Disabled() {
		t.Error("closing the approval window should restore the clock controls")
	}
}

func TestRefereeApproval_ClosingClockClosesApprovalWindow(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	openReferee(t, clk)

	clk.closeWindow()

	if clk.refereeWindow != nil {
		t.Error("closing the clock should also close the approval window")
	}
}

func TestScalingGridLayout_FontScalesWithWidth(t *testing.T) {
	var objs []fyne.CanvasObject
	for range 10 { // two rows of five
		objs = append(objs, approvalCell("Placeholder", theme.ColorNameBackground, false))
	}

	if got := (scalingGridLayout{}).MinSize(objs); got.Width <= 0 || got.Height <= 0 {
		t.Fatalf("MinSize = %v, want positive", got)
	}

	scalingGridLayout{}.Layout(objs, fyne.NewSize(480, 400))
	narrow := cellText(objs[0]).TextSize
	if narrow < refereeFontMin || narrow > refereeFontDesign {
		t.Errorf("narrow font = %v, want within [%v, %v]", narrow, refereeFontMin, refereeFontDesign)
	}

	scalingGridLayout{}.Layout(objs, fyne.NewSize(4000, 400))
	wide := cellText(objs[0]).TextSize
	if wide != refereeFontDesign {
		t.Errorf("wide font = %v, want it clamped up to %v", wide, refereeFontDesign)
	}
	if wide <= narrow {
		t.Errorf("font should grow with width: narrow %v, wide %v", narrow, wide)
	}
}
