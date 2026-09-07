package clock

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

func tp(t time.Time) *time.Time { return &t }

// openDerivingClock opens a finish-timer clock bound to both the finish log it
// writes and the start log it derives the winning time from.
func openDerivingClock(t *testing.T, s persona.Session, finish *store.FinishLog, start *store.StartLog) *Clock {
	t.Helper()
	app := test.NewTempApp(t)
	clk := NewClock(app, createTestRegattaData(), createTestRaceData()).
		WithFinishLog(s, finish).
		WithStartLog(start)
	clk.OpenRaceClock()
	t.Cleanup(func() { clk.window.Close() })
	return clk
}

func emptyFinish() *store.FinishLog { return &store.FinishLog{Races: map[int]store.RaceResult{}} }
func emptyStart() *store.StartLog   { return &store.StartLog{Races: map[int]store.StartRecord{}} }

func skewDismissButton(c *Clock) *widget.Button {
	for _, o := range c.skewBanner.Objects {
		if b, ok := o.(*widget.Button); ok {
			return b
		}
	}
	return nil
}

func TestDeriveWinningTimeOnStart(t *testing.T) {
	s := pftSession(t)
	finish := emptyFinish()
	start := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: tp(time.Now().UTC().Add(-6 * time.Minute))},
	}}
	clk := openDerivingClock(t, s, finish, start)

	clk.buttons.start.OnTapped()

	if clk.derivedWinningTime == common.EmptyString {
		t.Fatal("Start did not derive a winning time")
	}
	if clk.winningTime.Text != clk.derivedWinningTime {
		t.Errorf("field %q != derived %q", clk.winningTime.Text, clk.derivedWinningTime)
	}
	// ~6:00, with a little slack for test execution time.
	if clk.winningTime.Text < "05:59.0" || clk.winningTime.Text > "06:00.9" {
		t.Errorf("derived winning time = %q, want ~06:00.0", clk.winningTime.Text)
	}
	if finish.Races[1].StartedAt == nil {
		t.Error("derive should record the ST side used on the RaceResult, for later audit")
	}
}

func TestDeriveWaitsForStartTime(t *testing.T) {
	s := pftSession(t)
	clk := openDerivingClock(t, s, emptyFinish(), emptyStart())

	clk.buttons.start.OnTapped()

	if !clk.awaitingStart {
		t.Error("clock should be awaiting the ST start time")
	}
	if clk.winningTime.Text != common.EmptyString {
		t.Errorf("winning time = %q, want empty while waiting", clk.winningTime.Text)
	}
	if clk.winningTime.PlaceHolder != common.WaitingForStartTimeText {
		t.Errorf("placeholder = %q, want the waiting text", clk.winningTime.PlaceHolder)
	}
}

func TestDeriveImplausibleSuppressed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start time.Time
	}{
		{"future start (negative)", time.Now().UTC().Add(2 * time.Minute)},
		{"far past start (over 30m)", time.Now().UTC().Add(-40 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pftSession(t)
			start := &store.StartLog{Races: map[int]store.StartRecord{
				1: {RaceNumber: 1, StartedAt: tp(tc.start)},
			}}
			clk := openDerivingClock(t, s, emptyFinish(), start)

			clk.buttons.start.OnTapped()

			if clk.winningTime.Text != common.EmptyString {
				t.Errorf("winning time = %q, want empty for an implausible derive", clk.winningTime.Text)
			}
			if clk.derivedWinningTime != common.EmptyString {
				t.Errorf("derivedWinningTime = %q, want empty", clk.derivedWinningTime)
			}
			if clk.awaitingStart {
				t.Error("an implausible start time is not a waiting state")
			}
		})
	}
}

func TestUpdateStartTimeRecomputes(t *testing.T) {
	s := pftSession(t)
	finish := emptyFinish()
	clk := openDerivingClock(t, s, finish, emptyStart())

	clk.buttons.start.OnTapped()
	if !clk.awaitingStart {
		t.Fatal("precondition: should be waiting for the start time")
	}

	clk.UpdateStartTime(&store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: tp(time.Now().UTC().Add(-5 * time.Minute))},
	}})

	if clk.awaitingStart {
		t.Error("still waiting after the start time arrived")
	}
	if clk.winningTime.Text == common.EmptyString {
		t.Error("winning time did not recompute when the start time arrived")
	}
	onDisk, err := store.LoadFinish(s)
	if err != nil {
		t.Fatalf("LoadFinish: %v", err)
	}
	if onDisk.Races[1].StartedAt == nil {
		t.Error("recompute should have persisted the ST side to finish.json")
	}
}

func TestUpdateStartTimeRespectsManualEntry(t *testing.T) {
	s := pftSession(t)
	start := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: tp(time.Now().UTC().Add(-6 * time.Minute))},
	}}
	clk := openDerivingClock(t, s, emptyFinish(), start)
	clk.buttons.start.OnTapped()

	// Referee types the official time.
	clk.winningTime.SetText("05:42.3")

	clk.UpdateStartTime(&store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: tp(time.Now().UTC().Add(-9 * time.Minute))},
	}})

	if clk.winningTime.Text != "05:42.3" {
		t.Errorf("winning time = %q, want the referee's manual entry left untouched", clk.winningTime.Text)
	}
}

func TestSkewBannerShownAndDismissed(t *testing.T) {
	s := pftSession(t)
	finish := emptyFinish()
	finish.Machine = "ft-laptop"
	start := &store.StartLog{Races: map[int]store.StartRecord{
		1: {
			RaceNumber: 1,
			StartedAt:  tp(time.Now().UTC().Add(-6 * time.Minute)),
			Clock:      timesync.ClockRef{Offset: 3 * time.Second},
		},
	}}
	start.Machine = "st-laptop"
	clk := openDerivingClock(t, s, finish, start)

	clk.buttons.start.OnTapped()

	if clk.skewBanner.Hidden {
		t.Fatal("skew banner should be visible when the offsets disagree by > 1s")
	}
	if txt := clk.skewLabel.Text; !strings.Contains(txt, "ft-laptop") || !strings.Contains(txt, "st-laptop") {
		t.Errorf("skew banner %q should name both machines", txt)
	}

	dismiss := skewDismissButton(clk)
	if dismiss == nil {
		t.Fatal("no dismiss button on the skew banner")
	}
	dismiss.OnTapped()
	if !clk.skewBanner.Hidden {
		t.Error("dismiss should hide the banner")
	}

	// A later skew check must not bring a dismissed banner back.
	clk.checkSkew(timesync.ClockRef{Offset: 9 * time.Second}, timesync.ClockRef{})
	if !clk.skewBanner.Hidden {
		t.Error("a dismissed skew banner reappeared")
	}
}

func TestNoSkewBannerWhenOffsetsAgree(t *testing.T) {
	s := pftSession(t)
	start := &store.StartLog{Races: map[int]store.StartRecord{
		1: {
			RaceNumber: 1,
			StartedAt:  tp(time.Now().UTC().Add(-6 * time.Minute)),
			Clock:      timesync.ClockRef{Offset: 200 * time.Millisecond},
		},
	}}
	clk := openDerivingClock(t, s, emptyFinish(), start)

	clk.buttons.start.OnTapped()

	if !clk.skewBanner.Hidden {
		t.Error("skew banner should stay hidden when offsets are within threshold")
	}
}
