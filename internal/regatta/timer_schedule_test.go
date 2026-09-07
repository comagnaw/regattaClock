package regatta

import (
	"os"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// cloneSchedule deep-copies a schedule so a test can mutate one race's lanes
// without touching the original the session was built from.
func cloneSchedule(sch *store.Schedule) *store.Schedule {
	out := *sch
	out.Races = make([]store.ScheduleRace, len(sch.Races))
	for i, race := range sch.Races {
		race.Lanes = make(map[int]store.ScheduleEntry, len(sch.Races[i].Lanes))
		for lane, entry := range sch.Races[i].Lanes {
			race.Lanes[lane] = entry
		}
		out.Races[i] = race
	}
	return &out
}

func bannerDismissButton(c *fyne.Container) *widget.Button {
	for _, o := range c.Objects {
		if b, ok := o.(*widget.Button); ok {
			return b
		}
	}
	return nil
}

func TestScheduleChangeUntimedRaceIsSilent(t *testing.T) {
	r, sch, _ := startedTimer(t, "pst")

	next := cloneSchedule(sch)
	next.Races[0].Lanes[1] = store.ScheduleEntry{SchoolName: "Gamma"}
	r.onScheduleChanged(next)

	if len(r.scheduleConflicts) != 0 {
		t.Errorf("an untimed lane change raised a conflict: %v", r.scheduleConflicts)
	}
	if !r.scheduleBanner.Hidden {
		t.Error("banner should stay hidden for an untimed change")
	}
	if r.RegattaData.Races[0].Lanes[1].SchoolName != "Gamma" {
		t.Error("schedule mirror should still refresh silently")
	}
	if strings.HasPrefix(r.rows[1].title.Text, common.ScheduleConflictMark) {
		t.Error("no row mark for an untimed change")
	}
}

func TestScheduleChangeNotifiesStartTimerWithRecordedStart(t *testing.T) {
	r, sch, sess := startedTimer(t, "pst")
	r.recordStart(1)
	before, err := store.LoadStart(sess)
	if err != nil {
		t.Fatal(err)
	}

	next := cloneSchedule(sch)
	next.Races[0].Lanes[2] = store.ScheduleEntry{SchoolName: "Delta"}
	r.onScheduleChanged(next)

	if _, ok := r.scheduleConflicts[1]; !ok {
		t.Fatal("race 1 should be flagged: it has a recorded start")
	}
	if r.scheduleBanner.Hidden {
		t.Error("schedule banner should be visible")
	}
	if !strings.HasPrefix(r.rows[1].title.Text, common.ScheduleConflictMark) {
		t.Errorf("row title = %q, want the conflict mark", r.rows[1].title.Text)
	}

	after, err := store.LoadStart(sess)
	if err != nil {
		t.Fatal(err)
	}
	if after.Sequence != before.Sequence || after.Races[1].Display != before.Races[1].Display {
		t.Error("start.json must not be rewritten by a schedule change")
	}
}

func TestScheduleChangeNotifiesFinishTimerWithResults(t *testing.T) {
	r, sch, sess := startedTimer(t, "pft")
	ff := time.Now().UTC()
	r.finishLog.Races[1] = store.RaceResult{RaceNumber: 1, FirstFinishAt: &ff}

	next := cloneSchedule(sch)
	next.Races[0].Lanes[1] = store.ScheduleEntry{SchoolName: "Zeta"}
	r.onScheduleChanged(next)

	if _, ok := r.scheduleConflicts[1]; !ok {
		t.Fatal("a race with finish data should be flagged after a lane change")
	}
	if got := r.scheduleBannerLabel.Text; !strings.Contains(got, "Review order of finish") {
		t.Errorf("FT banner = %q, want the finish-timer copy", got)
	}
	if _, err := os.Stat(sess.FinishPath()); !os.IsNotExist(err) {
		t.Error("finish.json must not be written by a schedule change")
	}
}

func TestScheduleChangePushesToOpenFinishClock(t *testing.T) {
	r, sch, _ := startedTimer(t, "pft")
	r.openClock(1)
	if r.openClocks[1] == nil {
		t.Fatal("clock did not open for race 1")
	}

	next := cloneSchedule(sch)
	next.Races[0].Lanes[1] = store.ScheduleEntry{SchoolName: "Moved"}
	r.onScheduleChanged(next)

	if r.openClocks[1] == nil {
		t.Error("a schedule change must not close the open clock")
	}
	if _, ok := r.scheduleConflicts[1]; !ok {
		t.Error("a race with an open clock should be flagged")
	}
}

func TestScheduleBannerDismissClearsMarks(t *testing.T) {
	r, sch, _ := startedTimer(t, "pst")
	r.recordStart(1)

	next := cloneSchedule(sch)
	next.Races[0].Lanes[1] = store.ScheduleEntry{SchoolName: "New Crew"}
	r.onScheduleChanged(next)
	if r.scheduleBanner.Hidden {
		t.Fatal("precondition: banner visible")
	}

	dismiss := bannerDismissButton(r.scheduleBanner)
	if dismiss == nil {
		t.Fatal("no dismiss button on the schedule banner")
	}
	dismiss.OnTapped()

	if len(r.scheduleConflicts) != 0 {
		t.Error("dismiss should clear the conflict set")
	}
	if !r.scheduleBanner.Hidden {
		t.Error("dismiss should hide the banner")
	}
	if strings.HasPrefix(r.rows[1].title.Text, common.ScheduleConflictMark) {
		t.Error("dismiss should clear the row mark")
	}
}
