package clock

import (
	"slices"
	"strings"
	"testing"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

func TestPrimaryFinish_PanelHasRefereeAndClose(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})
	labels := buttonLabels(clk.approvalPanel())

	if !slices.Contains(labels, common.RefereeButtonText) || !slices.Contains(labels, common.CloseButtonText) {
		t.Errorf("primary approval panel should have Referee Approval + Close: %v", labels)
	}
	if slices.Contains(labels, common.SaveButtonText) || slices.Contains(labels, common.SaveAndCloseButtonText) {
		t.Errorf("primary approval panel should have no Save button: %v", labels)
	}
}

func TestPrimaryFinish_CloseDisabledUntilApproved(t *testing.T) {
	clk := openBoundClock(t, pftSession(t), &store.FinishLog{Races: map[int]store.RaceResult{}})

	if !clk.buttons.close.Disabled() {
		t.Fatal("Close should start disabled")
	}
	if clk.commitStatus.Text != common.CommitStatusPending {
		t.Errorf("commit status = %q, want %q", clk.commitStatus.Text, common.CommitStatusPending)
	}

	clk.buttons.start.OnTapped()
	clk.winningTime.SetText("01:00.0")
	if !clk.buttons.close.Disabled() {
		t.Error("Close stays disabled while the race is only Pending (winning time entered, not approved)")
	}
	if clk.buttons.referee.Disabled() {
		t.Error("a valid winning time should enable Referee Approval")
	}

	clk.refereeApprovalFunc(1)(true)

	if clk.buttons.close.Disabled() {
		t.Error("Close should be enabled once the race is approved")
	}
	if !strings.HasPrefix(clk.commitStatus.Text, "Approved ") {
		t.Errorf("commit status = %q, want an \"Approved …\" line", clk.commitStatus.Text)
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
