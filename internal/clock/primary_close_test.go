package clock

import (
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/uitheme"
)

func TestPrimaryFinish_PanelHasRefereeAndClose(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	labels := buttonLabels(clk.controlsAndApprovalPanel())

	if !slices.Contains(labels, common.RefereeButtonText) || !slices.Contains(labels, common.CloseButtonText) {
		t.Errorf("primary approval panel should have Referee Approval + Close: %v", labels)
	}
	if slices.Contains(labels, common.SaveButtonText) || slices.Contains(labels, common.SaveAndCloseButtonText) {
		t.Errorf("primary approval panel should have no Save button: %v", labels)
	}
}

// TestPrimaryFinish_CommitStatusOnAccentBand - the status line under the
// approval buttons should carry the same blue-band contrast as the rest of
// the window (docs/features/PRE-RELEASE-BUGS.md, Feature 2), not sit as a
// plain label on the window background.
func TestPrimaryFinish_CommitStatusOnAccentBand(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})

	found := false
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *canvas.Rectangle:
			if sameColor(v.FillColor, uitheme.LogoWaterBlue) {
				found = true
			}
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.ThemeOverride:
			walk(v.Content)
		}
	}
	walk(clk.controlsAndApprovalPanel())

	if !found {
		t.Error("approval panel should carry an AccentBand (LogoWaterBlue fill) under the commit status line")
	}
}

func TestPrimaryFinish_CloseDisabledUntilApproved(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})

	if !clk.buttons.close.Disabled() {
		t.Fatal("Close should start disabled")
	}
	wantPending := store.StateNotStarted.DisplayText(persona.TeamPrimary)
	if clk.commitStatus.Text != wantPending {
		t.Errorf("commit status = %q, want %q", clk.commitStatus.Text, wantPending)
	}

	clk.buttons.start.OnTapped()
	clk.winningTime.SetText("01:00.0")
	if !clk.buttons.close.Disabled() {
		t.Error("Close stays disabled while the race is only On the Water (winning time entered, not approved)")
	}
	if clk.buttons.referee.Disabled() {
		t.Error("a valid winning time should enable Referee Approval")
	}

	clk.refereeApprovalFunc(1)(true)

	if clk.buttons.close.Disabled() {
		t.Error("Close should be enabled once the race is approved")
	}
	if !strings.HasPrefix(clk.commitStatus.Text, "Approved on ") {
		t.Errorf("commit status = %q, want an 'Approved on …' line", clk.commitStatus.Text)
	}
}

func TestPrimaryFinish_ApproveKeepsWindowOpen(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	closed := false
	clk.AfterClose = func() { closed = true }

	clk.buttons.start.OnTapped()
	clk.winningTime.SetText("01:00.0")
	clk.refereeApprovalFunc(1)(true)

	if closed {
		t.Error("Referee Approval must not close the primary FT clock window")
	}
}

// TestPrimaryFinish_ApproveFiresOnCommit - Referee Approval leaves the clock
// window open (see TestPrimaryFinish_ApproveKeepsWindowOpen above), so the
// owning race tree can't rely on AfterClose to learn the result was saved.
// OnCommit is the live-refresh hook for that case; a regression here would
// leave the background race tree showing a stale (non-Official) status until
// the PFT operator eventually closes the clock.
func TestPrimaryFinish_ApproveFiresOnCommit(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	commits := 0
	clk.OnCommit = func() { commits++ }

	clk.buttons.start.OnTapped()
	clk.winningTime.SetText("01:00.0")
	clk.refereeApprovalFunc(1)(true)

	if commits != 1 {
		t.Errorf("OnCommit fire count = %d, want 1 after Referee Approval", commits)
	}
}
