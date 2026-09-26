package sample

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

// scheduledTimeLayout is the Heat Sheet's column-B time, e.g. "09:07 AM".
const scheduledTimeLayout = "03:04 PM"

// Each team's measured NTP offset. Start and finish on one team share it,
// so it cancels out of the winning time exactly as it does on race day.
var teamOffset = map[persona.Team]time.Duration{
	persona.TeamPrimary:   12 * time.Millisecond,
	persona.TeamSecondary: -7 * time.Millisecond,
}

// timing is the four synthesized timing logs for one sample.
type timing struct {
	start  map[persona.Team]*store.StartLog
	finish map[persona.Team]*store.FinishLog
}

// synthesize builds start and finish logs for every race in raced, dated
// day (local midnight of the regatta date):
//   - both start timers record the scheduled time plus a deterministic 0-90 s
//     late start, the secondary a few tenths off the primary;
//   - the primary finish timer's results are the fixture's, Approved;
//   - the secondary's are the same finish order a few tenths off, Saved.
func synthesize(sch *store.Schedule, raced []Race, day time.Time) (timing, error) {
	key := sch.Key()
	lanes := make(map[int]store.ScheduleRace, len(sch.Races))
	for _, r := range sch.Races {
		lanes[r.RaceNumber] = r
	}

	t := timing{
		start:  map[persona.Team]*store.StartLog{},
		finish: map[persona.Team]*store.FinishLog{},
	}
	for _, team := range []persona.Team{persona.TeamPrimary, persona.TeamSecondary} {
		t.start[team] = &store.StartLog{Races: map[int]store.StartRecord{}}
		t.finish[team] = &store.FinishLog{Races: map[int]store.RaceResult{}}
	}

	for _, race := range raced {
		sr, ok := lanes[race.RaceNumber]
		if !ok {
			return timing{}, fmt.Errorf("race %d is not on the generated schedule", race.RaceNumber)
		}
		sched, err := time.ParseInLocation(scheduledTimeLayout, race.ScheduledTime, day.Location())
		if err != nil {
			return timing{}, fmt.Errorf("race %d: scheduled time %q: %w", race.RaceNumber, race.ScheduledTime, err)
		}
		primaryStart := day.Add(time.Duration(sched.Hour())*time.Hour +
			time.Duration(sched.Minute())*time.Minute +
			tenths(jitter(race.RaceNumber, "late", 900)))
		secondaryStart := primaryStart.Add(tenths(jitter(race.RaceNumber, "sst", 9) - 4))

		rows, err := finishRows(race)
		if err != nil {
			return timing{}, fmt.Errorf("race %d: %w", race.RaceNumber, err)
		}
		// The secondary's stopwatch runs a few tenths off the primary's -
		// never identical, so a compare view has something to show.
		drift := tenths(jitter(race.RaceNumber, "sft", 5) + 1)
		if jitter(race.RaceNumber, "sign", 2) == 0 {
			drift = -drift
		}

		for team, start := range map[persona.Team]time.Time{
			persona.TeamPrimary:   primaryStart,
			persona.TeamSecondary: secondaryStart,
		} {
			ref := clockRef(team, primaryStart)
			started := start.UTC()
			t.start[team].Races[race.RaceNumber] = store.StartRecord{
				RaceNumber: race.RaceNumber,
				StartedAt:  &started,
				Display:    ref.Corrected(start).Format(common.StartTimeDisplayLayout),
				Clock:      ref,
			}

			teamRows := rows
			if team == persona.TeamSecondary {
				teamRows = shiftRows(rows, drift)
			}
			t.finish[team].Races[race.RaceNumber] = result(team, race.RaceNumber, start, ref, teamRows, sr.LaneMapHash())
		}
	}

	for team, log := range t.start {
		written := day
		for _, rec := range log.Races {
			if rec.StartedAt.After(written) {
				written = *rec.StartedAt
			}
		}
		stamp(&log.Envelope, definitionFor(persona.RoleStart, team), key, written, len(log.Races))
	}
	for team, log := range t.finish {
		written := day
		for _, res := range log.Races {
			if res.UpdatedAt.After(written) {
				written = res.UpdatedAt
			}
		}
		stamp(&log.Envelope, definitionFor(persona.RoleFinish, team), key, written, len(log.Races))
	}
	return t, nil
}

// result is one team's RaceResult for a race started at start. The primary's
// is Approved (Official); the secondary's is Saved - it has a winning time
// but no approval gate, and never a StoppedAt.
func result(team persona.Team, n int, start time.Time, ref timesync.ClockRef, rows []store.LapRow, laneMapHash string) store.RaceResult {
	winning, slowest := timeBounds(rows)
	firstFinish := start.Add(winning).UTC()
	stopped := start.Add(slowest + 5*time.Second).UTC()

	res := store.RaceResult{
		RaceNumber:       n,
		StartedAt:        new(start.UTC()),
		StartedAtClock:   ref,
		FirstFinishAt:    &firstFinish,
		FirstFinishClock: ref,
		Rows:             rows,
		LaneMapHash:      laneMapHash,
	}
	if winning > 0 {
		res.WinningTime = FormatClock(winning)
	}
	if team == persona.TeamPrimary {
		approved := stopped.Add(90*time.Second + tenths(jitter(n, "approve", 600)))
		res.StoppedAt = &stopped
		res.Approved = true
		res.ApprovedAt = &approved
		res.UpdatedAt = approved
	} else {
		res.UpdatedAt = stopped.Add(30 * time.Second)
	}
	return res
}

// finishRows is the fixture race's finish order as LapRows: numeric places
// first, ascending, then non-numeric ones (DNS, Excluded, ...). Ties - a dead
// heat, or two non-numeric places - go by lane, so the order never depends
// on map iteration. A lane with no Place never reached the finish line and
// has no row.
func finishRows(race Race) ([]store.LapRow, error) {
	var rows []store.LapRow
	for lane, l := range race.Lanes {
		if l.Place == "" {
			continue
		}
		for _, v := range []string{l.Split, l.Time} {
			if v == "" {
				continue
			}
			if _, err := ParseClock(v); err != nil {
				return nil, fmt.Errorf("lane %d: %w", lane, err)
			}
		}
		rows = append(rows, store.LapRow{Lane: lane, Place: l.Place, Split: l.Split, Time: l.Time})
	}
	sort.Slice(rows, func(i, j int) bool {
		pi, ei := strconv.Atoi(rows[i].Place)
		pj, ej := strconv.Atoi(rows[j].Place)
		switch {
		case ei == nil && ej == nil && pi != pj:
			return pi < pj
		case ei == nil && ej == nil:
			return rows[i].Lane < rows[j].Lane
		case ei == nil:
			return true
		case ej == nil:
			return false
		default:
			return rows[i].Lane < rows[j].Lane
		}
	})
	return rows, nil
}

// shiftRows moves every Time by d, keeping splits (the gaps behind the
// winner) as they are - the secondary's watch started a touch early or late.
func shiftRows(rows []store.LapRow, d time.Duration) []store.LapRow {
	out := make([]store.LapRow, len(rows))
	for i, r := range rows {
		out[i] = r
		if t, err := ParseClock(r.Time); err == nil && r.Time != "" {
			out[i].Time = FormatClock(t + d)
		}
	}
	return out
}

// timeBounds returns the winning (fastest) and slowest finish times.
func timeBounds(rows []store.LapRow) (winning, slowest time.Duration) {
	for _, r := range rows {
		t, err := ParseClock(r.Time)
		if err != nil || r.Time == "" {
			continue
		}
		if winning == 0 || t < winning {
			winning = t
		}
		if t > slowest {
			slowest = t
		}
	}
	return winning, slowest
}

// clockRef is a plausible NTP measurement taken shortly before the race.
func clockRef(team persona.Team, start time.Time) timesync.ClockRef {
	return timesync.ClockRef{
		Offset:     teamOffset[team],
		RTT:        18 * time.Millisecond,
		Source:     "ntp:sample",
		MeasuredAt: start.Add(-2 * time.Minute).UTC(),
	}
}

// stamp fills a timing file's envelope directly - the store.Save* path would
// stamp it too, but also stages the write through the local journal, which
// the generator must not touch.
func stamp(e *store.Envelope, def persona.Definition, regattaKey string, written time.Time, races int) {
	e.Version = store.SchemaVersion
	e.Role = def.Role
	e.Team = def.Team
	e.RegattaKey = regattaKey
	e.Machine = "SAMPLE-" + strings.ToUpper(def.ID)
	e.WrittenAt = written.UTC()
	e.Sequence = races
	e.Clock = clockRef(def.Team, written)
}

// definitionFor returns the persona that owns role+team's timing file.
func definitionFor(role persona.Role, team persona.Team) persona.Definition {
	for _, d := range persona.Registry {
		if d.Role == role && d.Team == team {
			return d
		}
	}
	panic(fmt.Sprintf("sample: no persona for %s/%s", role, team))
}

// jitter is a deterministic value in [0, n) for race n and a purpose, so a
// re-run of the generator writes the same timings.
func jitter(race int, purpose string, n int) int {
	h := fnv.New32a()
	fmt.Fprintf(h, "%d/%s", race, purpose)
	return int(h.Sum32() % uint32(n))
}

func tenths(n int) time.Duration { return time.Duration(n) * 100 * time.Millisecond }

// ParseClock parses the clock's "MM:SS.t" finish-time format - the shape of
// every fixture Split and Time.
func ParseClock(s string) (time.Duration, error) {
	bad := fmt.Errorf("finish time %q is not MM:SS.t", s)
	mm, rest, ok := strings.Cut(s, ":")
	if !ok {
		return 0, bad
	}
	ss, t, ok := strings.Cut(rest, ".")
	if !ok || len(t) != 1 {
		return 0, bad
	}
	var parts [3]int
	for i, p := range []string{mm, ss, t} {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return 0, bad
		}
		parts[i] = v
	}
	if parts[1] > 59 {
		return 0, bad
	}
	return time.Duration(parts[0])*time.Minute + time.Duration(parts[1])*time.Second + tenths(parts[2]), nil
}

// FormatClock formats d as the clock does (internal/clock's formatTime).
func FormatClock(d time.Duration) string {
	return fmt.Sprintf(common.ClockFormatter, int(d.Minutes())%60, int(d.Seconds())%60, int(d.Milliseconds()/100)%10)
}
