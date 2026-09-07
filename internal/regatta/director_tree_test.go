package regatta

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

func tm(min int) *time.Time { u := time.Now().UTC().Add(time.Duration(min) * time.Minute); return &u }

// directorWithTeamLogs builds a director bound to a two-race regatta with its
// rows realised, ready for the caller to poke r.teamLogs and refresh.
func directorWithTeamLogs(t *testing.T, primary, secondary *teamTiming) *Regatta {
	t.Helper()
	r := directorWithRaces(t, []reader.RaceData{
		{RaceNumber: 1, BoatCount: 4, Lanes: map[int]reader.RaceEntry{1: {SchoolName: "A"}}},
		{RaceNumber: 2, BoatCount: 4, Lanes: map[int]reader.RaceEntry{1: {SchoolName: "B"}}},
	})
	r.raceListBody() // realises r.rows
	r.teamLogs = map[persona.Team]*teamTiming{
		persona.TeamPrimary:   primary,
		persona.TeamSecondary: secondary,
	}
	r.refreshAllRows()
	return r
}

func startLogWith(recs map[int]store.StartRecord) *store.StartLog {
	return &store.StartLog{Races: recs}
}
func finishLogWith(recs map[int]store.RaceResult) *store.FinishLog {
	return &store.FinishLog{Races: recs}
}

func TestDirectorRow_PrimaryValues(t *testing.T) {
	r := directorWithTeamLogs(t,
		&teamTiming{
			start: startLogWith(map[int]store.StartRecord{
				1: {RaceNumber: 1, StartedAt: tm(-8), Display: "09:00:00.0", Cleared: []store.ClearedStart{{}, {}}},
			}),
			finish: finishLogWith(map[int]store.RaceResult{
				1: {RaceNumber: 1, WinningTime: "06:00.0", Approved: true},
			}),
		},
		&teamTiming{},
	)

	row := r.rows[1]
	if row.restarts.Text != "2" {
		t.Errorf("restarts = %q, want 2", row.restarts.Text)
	}
	if row.startTime.Text != "09:00:00.0" {
		t.Errorf("start = %q", row.startTime.Text)
	}
	if row.winTime.Text != "06:00.0" {
		t.Errorf("winning time = %q", row.winTime.Text)
	}
	if row.approved.Text != common.RaceApprovedText {
		t.Errorf("status = %q, want %q", row.approved.Text, common.RaceApprovedText)
	}
	if strings.Contains(row.startTime.Text+row.winTime.Text, common.SecondaryValueMark) {
		t.Error("primary values must not carry the secondary marker")
	}
}

func TestDirectorRow_SecondaryFallbackMarked(t *testing.T) {
	r := directorWithTeamLogs(t,
		// primary has race 1 only
		&teamTiming{
			start:  startLogWith(map[int]store.StartRecord{1: {RaceNumber: 1, StartedAt: tm(-5), Display: "10:00:00.0"}}),
			finish: finishLogWith(map[int]store.RaceResult{1: {RaceNumber: 1, WinningTime: "05:00.0"}}),
		},
		// secondary has race 2
		&teamTiming{
			start:  startLogWith(map[int]store.StartRecord{2: {RaceNumber: 2, StartedAt: tm(-3), Display: "10:30:00.0"}}),
			finish: finishLogWith(map[int]store.RaceResult{2: {RaceNumber: 2, WinningTime: "05:30.0", Approved: true}}),
		},
	)

	if got := r.rows[1].winTime.Text; got != "05:00.0" {
		t.Errorf("race 1 winning time = %q, want the primary value unmarked", got)
	}
	if got := r.rows[2].startTime.Text; got != "10:30:00.0"+common.SecondaryValueMark {
		t.Errorf("race 2 start = %q, want the secondary value marked", got)
	}
	if got := r.rows[2].winTime.Text; got != "05:30.0"+common.SecondaryValueMark {
		t.Errorf("race 2 winning time = %q, want the secondary value marked", got)
	}
	if got := r.rows[2].approved.Text; got != common.RaceApprovedText+common.SecondaryValueMark {
		t.Errorf("race 2 status = %q", got)
	}
}

func TestDirectorRow_InProgressStatus(t *testing.T) {
	r := directorWithTeamLogs(t,
		&teamTiming{finish: finishLogWith(map[int]store.RaceResult{
			1: {RaceNumber: 1, FirstFinishAt: tm(-1)}, // started, nothing saved
		})},
		&teamTiming{},
	)
	if got := r.rows[1].approved.Text; got != common.RaceInProgressText {
		t.Errorf("in-progress status = %q, want %q", got, common.RaceInProgressText)
	}
}

func TestDirectorRow_Placeholders(t *testing.T) {
	r := directorWithTeamLogs(t, &teamTiming{}, &teamTiming{})
	row := r.rows[1]
	if row.restarts.Text != common.NoStartTimeText || row.startTime.Text != common.NoStartTimeText ||
		row.winTime.Text != common.NoStartTimeText || row.approved.Text != common.EmptyString {
		t.Errorf("placeholders wrong: %q %q %q %q",
			row.restarts.Text, row.startTime.Text, row.winTime.Text, row.approved.Text)
	}
}

func TestDirectorHydratesBothTeams(t *testing.T) {
	app := test.NewTempApp(t)
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	pst := timerSession(t, "pst", root)
	ps := &store.StartLog{Races: map[int]store.StartRecord{1: {RaceNumber: 1, StartedAt: tm(-6), Display: "08:00:00.0"}}}
	ps.RegattaKey = key
	if err := store.SaveStart(pst, ps); err != nil {
		t.Fatal(err)
	}
	sft := timerSession(t, "sft", root)
	sf := &store.FinishLog{Races: map[int]store.RaceResult{2: {RaceNumber: 2, WinningTime: "07:00.0"}}}
	sf.RegattaKey = key
	if err := store.SaveFinish(sft, sf); err != nil {
		t.Fatal(err)
	}

	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))
	r := NewDirector(app)
	stopWatch(t, r)

	if tt := r.teamLogs[persona.TeamPrimary]; tt == nil || tt.start.Races[1].StartedAt == nil {
		t.Fatal("primary start.json not hydrated")
	}
	if tt := r.teamLogs[persona.TeamSecondary]; tt == nil || tt.finish.Races[2].WinningTime != "07:00.0" {
		t.Fatal("secondary finish.json not hydrated")
	}
	if r.rows[1].startTime.Text != "08:00:00.0" {
		t.Errorf("race 1 start = %q", r.rows[1].startTime.Text)
	}
	if r.rows[2].winTime.Text != "07:00.0"+common.SecondaryValueMark {
		t.Errorf("race 2 winning time = %q, want the secondary value marked", r.rows[2].winTime.Text)
	}
}

func TestDirectorTeamChangeRefreshesRow(t *testing.T) {
	r := directorWithTeamLogs(t, &teamTiming{}, &teamTiming{})
	if r.rows[1].startTime.Text != common.NoStartTimeText {
		t.Fatal("precondition")
	}

	r.onDirectorTeamChanged(persona.TeamPrimary,
		startLogWith(map[int]store.StartRecord{1: {RaceNumber: 1, StartedAt: tm(-2), Display: "11:11:11.1"}}), nil)

	if r.rows[1].startTime.Text != "11:11:11.1" {
		t.Errorf("row not refreshed after a watched team change: %q", r.rows[1].startTime.Text)
	}
}

func bannerVisible(b *dismissibleBanner) bool { return b != nil && !b.root.Hidden }

func bannerDismiss(b *dismissibleBanner) {
	for _, o := range b.root.Objects {
		if btn, ok := o.(*widget.Button); ok {
			btn.OnTapped()
			return
		}
	}
}

func envWithOffset(machine string, off time.Duration) store.Envelope {
	return store.Envelope{Machine: machine, Clock: timesync.ClockRef{Offset: off, Source: "ntp:test"}}
}

func TestDirectorSkewBanner(t *testing.T) {
	r := directorWithTeamLogs(t, &teamTiming{}, &teamTiming{})
	r.directorSkew = newDismissibleBanner()

	ps := &store.StartLog{}
	ps.Envelope = envWithOffset("laptop-a", 0)
	pf := &store.FinishLog{}
	pf.Envelope = envWithOffset("laptop-b", 3*time.Second)
	r.teamLogs[persona.TeamPrimary] = &teamTiming{start: ps, finish: pf}

	r.checkDirectorSkew()
	if !bannerVisible(r.directorSkew) {
		t.Fatal("skew banner should show for a 3s offset gap")
	}
	if txt := r.directorSkew.label.Text; !strings.Contains(txt, "laptop-a") || !strings.Contains(txt, "laptop-b") {
		t.Errorf("skew banner %q should name both machines", txt)
	}

	bannerDismiss(r.directorSkew)
	if bannerVisible(r.directorSkew) {
		t.Error("dismiss should hide the skew banner")
	}
	r.checkDirectorSkew()
	if bannerVisible(r.directorSkew) {
		t.Error("a dismissed skew banner must not reappear")
	}
}

func TestDirectorSkewBannerHiddenWhenAligned(t *testing.T) {
	r := directorWithTeamLogs(t, &teamTiming{}, &teamTiming{})
	r.directorSkew = newDismissibleBanner()

	ps := &store.StartLog{}
	ps.Envelope = envWithOffset("laptop-a", 100*time.Millisecond)
	pf := &store.FinishLog{}
	pf.Envelope = envWithOffset("laptop-b", 250*time.Millisecond)
	r.teamLogs[persona.TeamPrimary] = &teamTiming{start: ps, finish: pf}

	r.checkDirectorSkew()
	if bannerVisible(r.directorSkew) {
		t.Error("no skew banner when offsets agree within the threshold")
	}
}

func TestDirectorStaleBanner(t *testing.T) {
	r := directorWithTeamLogs(t, &teamTiming{}, &teamTiming{})
	r.directorStale = newDismissibleBanner()

	old := &store.StartLog{}
	old.Envelope.WrittenAt = time.Now().UTC().Add(-15 * time.Minute)
	r.teamLogs[persona.TeamPrimary] = &teamTiming{start: old}

	r.checkDirectorStale()
	if !bannerVisible(r.directorStale) {
		t.Fatal("stale banner should show when the last write is 15m old")
	}

	fresh := &store.StartLog{}
	fresh.Envelope.WrittenAt = time.Now().UTC()
	r.teamLogs[persona.TeamPrimary].start = fresh
	r.checkDirectorStale()
	if bannerVisible(r.directorStale) {
		t.Error("stale banner should clear once a fresh write lands")
	}
}

func TestDirectorHeaderExtrasHasLegend(t *testing.T) {
	r := directorWithRaces(t, nil)
	r.teamLogs = map[persona.Team]*teamTiming{}
	extras := r.directorHeaderExtras()
	if !hasLabelText(extras, common.SecondaryValueLegend) {
		t.Error("director header is missing the secondary-value legend")
	}
}

func hasLabelText(o fyne.CanvasObject, want string) bool {
	switch v := o.(type) {
	case *widget.Label:
		return v.Text == want
	case *fyne.Container:
		for _, c := range v.Objects {
			if hasLabelText(c, want) {
				return true
			}
		}
	}
	return false
}
