package clock

import (
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

func secResult(win string) *store.FinishLog {
	return &store.FinishLog{Races: map[int]store.RaceResult{
		1: {
			RaceNumber:  1,
			WinningTime: win,
			Rows: []store.LapRow{
				{Lane: 2, Place: "1", Split: "00:00.0", Time: win},
				{Lane: 4, Place: "2", Split: "00:03.1", Time: "x"},
				{}, {}, {}, {},
			},
		},
	}}
}

func openPFTWithSecondary(t *testing.T, own, sec *store.FinishLog) *Clock {
	t.Helper()
	app := test.NewTempApp(t)
	clk := NewClock(app, createTestRegattaData(), createTestRaceData()).
		WithFinishLog(pftSession(t), own).
		WithSecondaryFinish(sec)
	clk.OpenRaceClock()
	t.Cleanup(clk.closeWindow)
	return clk
}

// countEntries walks the tree and counts editable widgets.
func countEntries(o fyne.CanvasObject) (entries, enabledButtons int) {
	switch v := o.(type) {
	case *widget.Entry:
		if !v.Disabled() {
			entries++
		}
	case *widget.Button:
		if !v.Disabled() {
			enabledButtons++
		}
	case *fyne.Container:
		for _, c := range v.Objects {
			e, b := countEntries(c)
			entries += e
			enabledButtons += b
		}
	case *container.ThemeOverride:
		return countEntries(v.Content)
	}
	return
}

func labelTexts(o fyne.CanvasObject) []string {
	var out []string
	switch v := o.(type) {
	case *widget.Label:
		out = append(out, v.Text)
	case *fyne.Container:
		for _, c := range v.Objects {
			out = append(out, labelTexts(c)...)
		}
	case *container.ThemeOverride:
		out = append(out, labelTexts(v.Content)...)
	}
	return out
}

func TestCompareButton_OnlyForPrimaryFinishWithSecondary(t *testing.T) {
	pft := openPFTWithSecondary(t, emptyFinish(), secResult("05:00.0"))
	if !slices.Contains(buttonLabels(pft.approvalPanel()), common.CompareSecondaryButtonText) {
		t.Error("PFT with a secondary mirror should show the Compare Secondary button")
	}

	// PFT with no secondary mirror at all: no button in the panel.
	noSec := openBoundClock(t, pftSession(t), emptyFinish())
	if slices.Contains(buttonLabels(noSec.approvalPanel()), common.CompareSecondaryButtonText) {
		t.Error("PFT without a secondary mirror should not show the button")
	}

	// Secondary FT never sees it.
	sft := openSecondaryClock(t, sftSession(t), emptyFinish())
	if slices.Contains(buttonLabels(sft.approvalPanel()), common.CompareSecondaryButtonText) {
		t.Error("the secondary FT must not get a Compare Secondary button")
	}
}

func TestCompareButton_EnabledOnlyWithCommittedSecondaryResult(t *testing.T) {
	// Secondary mirror present but no result for the race.
	pft := openPFTWithSecondary(t, emptyFinish(), emptyFinish())
	if !pft.buttons.compare.Disabled() {
		t.Error("no secondary result -> Compare Secondary stays disabled")
	}

	// A record with no winning time is not yet comparable.
	pft.UpdateSecondaryFinish(&store.FinishLog{Races: map[int]store.RaceResult{1: {RaceNumber: 1}}})
	if !pft.buttons.compare.Disabled() {
		t.Error("secondary record without a winning time -> still disabled")
	}

	pft.UpdateSecondaryFinish(secResult("05:00.0"))
	if pft.buttons.compare.Disabled() {
		t.Error("a committed secondary winning time -> Compare Secondary enables")
	}
}

// comparePane returns the content of the open Compare Secondary window.
func comparePane(t *testing.T, c *Clock) fyne.CanvasObject {
	t.Helper()
	if c.compareWindow == nil {
		t.Fatal("compare window is not open")
	}
	return c.compareWindow.Content()
}

func TestCompareView_OpensReadOnlyPane(t *testing.T) {
	pft := openPFTWithSecondary(t, emptyFinish(), secResult("05:00.0"))
	stopClockTicker(t, pft)

	pft.toggleCompareSecondary()
	pane := comparePane(t, pft)

	// The pane shows the secondary's numbers.
	texts := strings.Join(labelTexts(pane), " ")
	for _, want := range []string{"05:00.0", "1", "2"} {
		if !strings.Contains(texts, want) {
			t.Errorf("compare pane is missing %q; got %q", want, texts)
		}
	}

	// ...and carries no editable inputs or live controls.
	entries, enabled := countEntries(pane)
	if entries != 0 {
		t.Errorf("compare pane has %d editable entries, want 0", entries)
	}
	if enabled != 0 {
		t.Errorf("compare pane has %d enabled buttons, want 0", enabled)
	}
	for _, banned := range []string{
		common.StartButtonText, common.LapButtonText, common.StopButtonText, "Clear",
		common.RefereeButtonText, common.CloseButtonText, common.SaveAndCloseButtonText,
		common.CompareSecondaryButtonText,
	} {
		if slices.Contains(buttonLabels(pane), banned) {
			t.Errorf("compare pane must not contain the %q control", banned)
		}
	}
}

func TestCompareView_ClockStaysLiveAndToggles(t *testing.T) {
	pft := openPFTWithSecondary(t, emptyFinish(), secResult("05:00.0"))
	stopClockTicker(t, pft)

	pft.toggleCompareSecondary()
	if pft.compareWindow == nil {
		t.Fatal("toggle should open the compare window")
	}
	if pft.buttons.start.Disabled() || pft.buttons.lap.Disabled() {
		t.Error("opening the compare window must not disable the clock's own run controls")
	}
	pft.buttons.start.OnTapped()
	if !pft.clockState.isRunning {
		t.Error("Start still works while the compare window is open")
	}
	pft.clockState.isRunning = false

	pft.toggleCompareSecondary()
	if pft.compareWindow != nil {
		t.Fatal("second toggle should close the compare window")
	}
	if pft.buttons.compare.Text != common.CompareSecondaryButtonText {
		t.Errorf("button label after close = %q, want %q", pft.buttons.compare.Text, common.CompareSecondaryButtonText)
	}
}

// Closing the clock while the compare window is open must not try to raise the
// clock again from the compare window's SetOnClosed (the clockClosed guard).
func TestCompareView_ClosingClockWithCompareOpenIsClean(t *testing.T) {
	pft := openPFTWithSecondary(t, emptyFinish(), secResult("05:00.0"))
	stopClockTicker(t, pft)
	pft.toggleCompareSecondary()
	if pft.compareWindow == nil {
		t.Fatal("precondition: compare window open")
	}

	pft.closeWindow() // clock teardown -> closes the compare window too
	if pft.compareWindow != nil {
		t.Error("closing the clock should close the compare window")
	}
}

func TestCompareView_LiveRefresh(t *testing.T) {
	pft := openPFTWithSecondary(t, emptyFinish(), secResult("05:00.0"))
	stopClockTicker(t, pft)
	pft.toggleCompareSecondary()

	pft.UpdateSecondaryFinish(secResult("04:59.9"))
	if pft.compareWindow == nil {
		t.Fatal("a live secondary update should keep the window open")
	}
	if !strings.Contains(strings.Join(labelTexts(comparePane(t, pft)), " "), "04:59.9") {
		t.Error("the open compare window should show the refreshed secondary winning time")
	}

	// The secondary result losing its winning time closes the window.
	pft.UpdateSecondaryFinish(emptyFinish())
	if pft.compareWindow != nil {
		t.Error("the window should close when the secondary result is no longer comparable")
	}
}

func TestCompareView_SkewNote(t *testing.T) {
	own := emptyFinish()
	own.Envelope.Machine = "ft-a"
	own.Envelope.Clock = timesync.ClockRef{Offset: 0, Source: "ntp:test"}

	sec := secResult("05:00.0")
	sec.Envelope.Machine = "ft-b"
	sec.Envelope.Clock = timesync.ClockRef{Offset: 3 * time.Second, Source: "ntp:test"}

	pft := openPFTWithSecondary(t, own, sec)
	stopClockTicker(t, pft)
	pft.toggleCompareSecondary()

	joined := strings.Join(labelTexts(comparePane(t, pft)), " ")
	if !strings.Contains(joined, "ft-a") || !strings.Contains(joined, "ft-b") {
		t.Errorf("a 3s machine-clock gap should show the skew note naming both machines; got %q", joined)
	}

	// Within threshold: no note.
	sec.Envelope.Clock = timesync.ClockRef{Offset: 200 * time.Millisecond, Source: "ntp:test"}
	pft.UpdateSecondaryFinish(sec)
	if strings.Contains(strings.Join(labelTexts(comparePane(t, pft)), " "), "clocks differ by") {
		t.Error("no skew note when the machine clocks agree within the threshold")
	}
}
