// Package publish builds the read-only, publish-ready view of an approved
// race result that every downstream content persona (Results Publisher,
// Social Media, Streamer) renders from - sketched in sidecar-personas.md's
// "Phase 0a". A leaf package: it reads store.Schedule/store.FinishLog and
// reader.RaceData's own title logic, and never writes anything, imports
// Fyne, or imports internal/regatta.
package publish

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// Row is one lane's finish-order line, joined from RaceResult.Rows (the
// timing data) and ScheduleRace.Lanes (the school/additional-info) by lane
// number. Lane is a display string, not the join key itself - "" when the
// underlying LapRow has no lane assignment (Lane == 0).
type Row struct {
	Place, Lane, School, AdditionalInfo, Time string
}

// PublishableRace is the publish-ready view of one approved race - every
// field reproducible from finish.json + regattaSchedule.json, so it is a
// cache, never an authority (future-result-driven-persona.md).
type PublishableRace struct {
	RaceNumber                     int
	Title, RegattaKey, WinningTime string
	Rows                           []Row
	ApprovedAt, SourceUpdatedAt    time.Time
	LaneMapHash, Revision          string
}

// BuildView joins every approved race in fin against sch, in ascending race
// order. A race with no schedule entry, or that is not yet Approved, is
// left out entirely - there is nothing publish-ready about it yet.
func BuildView(sch *store.Schedule, fin *store.FinishLog) []PublishableRace {
	if sch == nil || fin == nil {
		return nil
	}

	schedule := make(map[int]store.ScheduleRace, len(sch.Races))
	for _, race := range sch.Races {
		schedule[race.RaceNumber] = race
	}
	regattaKey := store.RegattaKey(sch.Name, sch.Date)

	var out []PublishableRace
	for n, res := range fin.Races {
		if !res.Approved {
			continue
		}
		sr, ok := schedule[n]
		if !ok {
			continue // race removed/renamed since approval - nothing to join against
		}

		var approvedAt time.Time
		if res.ApprovedAt != nil {
			approvedAt = *res.ApprovedAt
		}

		pr := PublishableRace{
			RaceNumber:      n,
			Title:           raceTitle(sr),
			RegattaKey:      regattaKey,
			WinningTime:     res.WinningTime,
			Rows:            joinRows(res.Rows, sr.Lanes),
			ApprovedAt:      approvedAt,
			SourceUpdatedAt: res.UpdatedAt,
			LaneMapHash:     res.LaneMapHash,
		}
		pr.Revision = Revision(pr)
		out = append(out, pr)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].RaceNumber < out[j].RaceNumber })
	return out
}

// raceTitle reuses reader.RaceData.RaceTitle() rather than re-deriving the
// same "Race N - class - flight" format independently.
func raceTitle(sr store.ScheduleRace) string {
	rd := reader.RaceData{RaceNumber: sr.RaceNumber, BoatClass: sr.BoatClass, FlightInfo: sr.FlightInfo}
	return rd.RaceTitle()
}

// joinRows maps each LapRow to a Row, filling School/AdditionalInfo from the
// schedule's lane map when the lane has an entry there (a row-level join
// miss - e.g. an unassigned Lane == 0, or a lane no longer on the schedule -
// leaves them empty, but the row itself is still included: its place/time
// are still real data).
func joinRows(rows []store.LapRow, lanes map[int]store.ScheduleEntry) []Row {
	out := make([]Row, 0, len(rows))
	for _, lr := range rows {
		row := Row{Place: lr.Place, Time: lr.Time}
		if lr.Lane != 0 {
			row.Lane = strconv.Itoa(lr.Lane)
			if e, ok := lanes[lr.Lane]; ok {
				row.School = e.SchoolName
				row.AdditionalInfo = e.AdditionalInfo
			}
		}
		out = append(out, row)
	}
	return out
}

// Revision hashes only the fields a reader would see - place/lane/school/time
// per row (in stable lane order, so out-of-order input rows still hash
// identically), plus winningTime and title. An incidental write that changes
// nothing visible (e.g. UpdatedAt alone) must not move it.
func Revision(pr PublishableRace) string {
	rows := append([]Row(nil), pr.Rows...)
	sort.Slice(rows, func(i, j int) bool { return laneSortKey(rows[i].Lane) < laneSortKey(rows[j].Lane) })

	var b strings.Builder
	fmt.Fprintf(&b, "%s\x00%s\x1d", pr.Title, pr.WinningTime)
	for _, row := range rows {
		fmt.Fprintf(&b, "%s\x1e%s\x1e%s\x1e%s\x1d", row.Place, row.Lane, row.School, row.Time)
	}
	return filesystem.HashBytes([]byte(b.String()))[:16]
}

// laneSortKey parses a Row's display Lane back to its numeric sort order; a
// blank (unassigned) lane sorts first, matching its "no lane" meaning.
func laneSortKey(lane string) int {
	n, err := strconv.Atoi(lane)
	if err != nil {
		return 0
	}
	return n
}

// RenderText produces the plaintext table every content persona's own
// render (clipboard, file, or the basis for an exporter-rendered image)
// starts from - the "Target artifact" in future-result-driven-persona.md.
func RenderText(r PublishableRace) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s Results\n", r.Title)
	for _, row := range r.Rows {
		school := row.School
		if school == "" {
			school = "—"
		}
		fmt.Fprintf(&b, "%s - %s %s\n", row.Place, school, row.Time)
	}
	return b.String()
}

// IsStale reports whether pr has changed since it was last published, per
// published - the {raceNumber: Revision} map a content persona keeps of what
// it last rendered (race-state-machine.md).
func IsStale(published map[int]string, pr PublishableRace) bool {
	return published[pr.RaceNumber] != Revision(pr)
}
