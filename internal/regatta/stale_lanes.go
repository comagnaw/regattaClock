package regatta

import (
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// staleLaneMap reports whether race n has a committed RaceResult whose stored
// LaneMapHash no longer matches the live schedule - results entered against an
// earlier lane map (persona-plan.md 3c item 4). Checked for the finish timer's
// own log and, for the director, either team's.
func (r *Regatta) staleLaneMap(n int, race reader.RaceData) bool {
	live := raceLaneMapHash(race)
	switch r.session.Role {
	case persona.RoleFinish:
		return finishResultStale(r.finishLog, n, live)
	case persona.RoleDirector:
		for _, team := range directorTeams {
			if tt := r.teamLogs[team]; tt != nil && finishResultStale(tt.finish, n, live) {
				return true
			}
		}
	}
	return false
}

// finishResultStale is the per-log test: a result exists, it has a stamp (a
// pre-8d result has none, so it is never flagged), the results were committed
// (winning time or approval), and the stamp differs from the live lane map.
func finishResultStale(log *store.FinishLog, n int, liveHash string) bool {
	if log == nil {
		return false
	}
	res, ok := log.Races[n]
	if !ok || res.LaneMapHash == common.EmptyString {
		return false
	}
	if res.WinningTime == common.EmptyString && !res.Approved {
		return false // in progress - not committed against anything yet
	}
	return res.LaneMapHash != liveHash
}

// refreshStaleLaneLegend shows the caution strip when any visible row carries
// the mark, unless the operator has dismissed it (mirrors
// refreshSecondaryValueLegend). The strip is created in showRaceTree.
func (r *Regatta) refreshStaleLaneLegend() {
	if r.staleLaneLegend == nil {
		return
	}
	for n := range r.rows {
		if race, ok := r.raceByNumber(n); ok && r.staleLaneMap(n, race) {
			r.staleLaneLegend.show(common.StaleLaneMapLegend)
			return
		}
	}
	r.staleLaneLegend.hide()
}
