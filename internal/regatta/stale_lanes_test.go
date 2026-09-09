package regatta

import (
	"strings"
	"testing"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// liveHash is the current lane-map hash for race n in r.RegattaData.
func liveHash(t *testing.T, r *Regatta, n int) string {
	t.Helper()
	race, ok := r.raceByNumber(n)
	if !ok {
		t.Fatalf("no race %d in RegattaData", n)
	}
	return raceLaneMapHash(race)
}

func TestFinishTree_StaleLaneMapMark(t *testing.T) {
	r, _, _ := startedTimer(t, "pft")

	// A committed result stamped against a lane map that no longer matches.
	r.finishLog.Races[1] = store.RaceResult{
		RaceNumber: 1, WinningTime: "06:00.0", Approved: true,
		LaneMapHash: "stale-hash-1234",
	}
	r.refreshAllRows()

	if !strings.HasPrefix(r.rows[1].title.Text, common.StaleLaneMapMark) {
		t.Errorf("row 1 title = %q, want the stale-lane mark", r.rows[1].title.Text)
	}
	if !bannerVisible(r.staleLaneLegend) {
		t.Error("the legend should show when a row is flagged")
	}

	// Re-stamp with the current lane map: the mark clears.
	res := r.finishLog.Races[1]
	res.LaneMapHash = liveHash(t, r, 1)
	r.finishLog.Races[1] = res
	r.refreshAllRows()

	if strings.HasPrefix(r.rows[1].title.Text, common.StaleLaneMapMark) {
		t.Errorf("row 1 title = %q, want the mark cleared", r.rows[1].title.Text)
	}
	if bannerVisible(r.staleLaneLegend) {
		t.Error("the legend should hide once no row is flagged")
	}
}

func TestFinishTree_StaleLaneMapNotFlaggedWhen(t *testing.T) {
	r, _, _ := startedTimer(t, "pft")

	cases := map[string]store.RaceResult{
		"pre-8d result has no stamp":   {RaceNumber: 1, WinningTime: "06:00.0", Approved: true, LaneMapHash: ""},
		"in progress, not committed":   {RaceNumber: 1, FirstFinishAt: tm(-1), LaneMapHash: "stale-hash"},
		"committed against live lanes": {RaceNumber: 1, WinningTime: "06:00.0"},
	}
	// fill in the live hash for the last case
	live := liveHash(t, r, 1)
	cases["committed against live lanes"] = store.RaceResult{RaceNumber: 1, WinningTime: "06:00.0", LaneMapHash: live}

	for name, res := range cases {
		t.Run(name, func(t *testing.T) {
			r.finishLog.Races[1] = res
			r.refreshAllRows()
			if strings.HasPrefix(r.rows[1].title.Text, common.StaleLaneMapMark) {
				t.Errorf("row 1 unexpectedly flagged: %q", r.rows[1].title.Text)
			}
		})
	}
}

func TestDirectorTree_StaleLaneMapMark_EitherTeam(t *testing.T) {
	r := directorWithTeamLogs(t,
		&teamTiming{}, // primary: no finish data
		&teamTiming{finish: finishLogWith(map[int]store.RaceResult{
			1: {RaceNumber: 1, WinningTime: "05:30.0", LaneMapHash: "stale-hash"},
		})},
	)
	r.refreshAllRows()

	if !strings.HasPrefix(r.rows[1].title.Text, common.StaleLaneMapMark) {
		t.Errorf("row 1 title = %q, want the mark (secondary team's result is stale)", r.rows[1].title.Text)
	}

	// Fix the secondary stamp -> mark clears.
	res := r.teamLogs[persona.TeamSecondary].finish.Races[1]
	res.LaneMapHash = liveHash(t, r, 1)
	r.teamLogs[persona.TeamSecondary].finish.Races[1] = res
	r.refreshAllRows()
	if strings.HasPrefix(r.rows[1].title.Text, common.StaleLaneMapMark) {
		t.Errorf("row 1 still flagged after re-stamp: %q", r.rows[1].title.Text)
	}
}

func TestStaleLaneMap_StartTimerNeverFlagged(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")
	// A start timer has no finish log; the mark logic must ignore it.
	race, _ := r.raceByNumber(1)
	if r.staleLaneMap(1, race) {
		t.Error("the start timer tree must never carry the stale-lane mark")
	}
}
