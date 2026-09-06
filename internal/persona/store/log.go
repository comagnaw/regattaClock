package store

import (
	"errors"
	"io/fs"
	"time"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

// MaxClearedPerRace caps the per-race cleared-start history. A race restarting
// more than a handful of times is not worth unbounded file growth
// (persona-plan.md section 4).
const MaxClearedPerRace = 10

// ErrWrongPersona is returned when a Save is asked to write a file the session's
// persona does not own.
var ErrWrongPersona = errors.New("store: persona may not write this file")

// Envelope is the header on every persona-owned timing file. Its fields let a
// reader reject a file written for a different regatta and estimate the
// writer's clock offset.
type Envelope struct {
	Version    int
	Role       persona.Role
	Team       persona.Team
	RegattaKey string    // RegattaKey of the schedule this file belongs to
	Machine    string    // hostname, for skew and conflict messages
	WrittenAt  time.Time // writer's wall clock at the last write, UTC
	Sequence   int       // monotonic per writer; guards against stale reads

	// Clock is the writer's measured offset at write time, kept for the skew
	// banner. Correctness uses the per-record ClockRef instead.
	Clock timesync.ClockRef
}

// StartLog is the whole of one team's start times. Its payload is the entire
// Races map for the regatta, never a partial one (persona-plan.md section 5b).
type StartLog struct {
	Envelope
	Races map[int]StartRecord // keyed by race number
}

// StartRecord is one race's start time plus the non-destructive history of
// values the start timer has cleared.
type StartRecord struct {
	RaceNumber int
	StartedAt  *time.Time // nil when never recorded or currently cleared
	Display    string     // "HH:MM:SS.m" as shown in the ST race tree
	Clock      timesync.ClockRef

	Cleared []ClearedStart // oldest first; capped at MaxClearedPerRace
}

// ClearedStart is a start time the ST discarded, retained so a mistaken clear
// can be restored with the clock offset that was in force when it was captured.
type ClearedStart struct {
	StartedAt time.Time
	Display   string
	Clock     timesync.ClockRef
	ClearedAt time.Time
}

// FinishLog is the whole of one team's race results.
type FinishLog struct {
	Envelope
	Races map[int]RaceResult
}

// RaceResult carries everything needed to rehydrate the clock window exactly as
// the finish timer left it.
type RaceResult struct {
	RaceNumber int

	// StartedAt is a copy of the ST record actually used, with the ST's offset,
	// so the winning time can be recomputed or audited later.
	StartedAt      *time.Time
	StartedAtClock timesync.ClockRef

	// FirstFinishAt is the FT's Start click - the first boat crossing the line
	// in this app, not the beginning of the race.
	FirstFinishAt    *time.Time
	FirstFinishClock timesync.ClockRef

	WinningTime string // referee time; auto-filled but user-editable
	Rows        []LapRow
	Approved    bool
	ApprovedAt  *time.Time
	UpdatedAt   time.Time
}

// LapRow is one finish-order row. It is a one-to-one mirror of the in-memory
// lapRow in internal/clock so save and restore are field copies.
type LapRow struct {
	Lane  int    // OOF lane assignment; 0 = unassigned
	Place string // "1".."6" or DQ / DNF / DNS
	Split string
	Time  string
}

// TrimCleared drops the oldest cleared-start entries beyond MaxClearedPerRace.
func (r *StartRecord) TrimCleared() {
	if len(r.Cleared) > MaxClearedPerRace {
		r.Cleared = r.Cleared[len(r.Cleared)-MaxClearedPerRace:]
	}
}

// LoadStart reads this session's team's start.json - the ST's own file, or the
// peer file a FT reads for start times. Missing file: errors.Is(err,
// fs.ErrNotExist). Unparseable: errors.Is(err, ErrCorrupt).
func LoadStart(s persona.Session) (*StartLog, error) {
	return loadLog[StartLog](s.StartPath(), "start")
}

// LoadFinish reads this session's team's finish.json.
func LoadFinish(s persona.Session) (*FinishLog, error) {
	return loadLog[FinishLog](s.FinishPath(), "finish")
}

// SaveStart atomically writes the whole StartLog to the ST's start.json. The
// caller updates only log.Races[n] in memory and passes the full map; this
// stamps the envelope and bumps the sequence.
func SaveStart(s persona.Session, log *StartLog) error {
	if s.Role != persona.RoleStart {
		return ErrWrongPersona
	}
	stampEnvelope(&log.Envelope, s)
	return saveLog(s.WritePath(), "start", log, len(log.Races))
}

// SaveFinish atomically writes the whole FinishLog to the FT's finish.json.
func SaveFinish(s persona.Session, log *FinishLog) error {
	if s.Role != persona.RoleFinish {
		return ErrWrongPersona
	}
	stampEnvelope(&log.Envelope, s)
	return saveLog(s.WritePath(), "finish", log, len(log.Races))
}

func loadLog[T any](path, kind string) (*T, error) {
	var log T
	if err := loadJSON(path, &log); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			applog.Error(kind+" log load failed", "component", "store", "file", path, "err", err)
		}
		return nil, err
	}
	applog.Info(kind+" log hydrated", "component", "store", "file", path)
	return &log, nil
}

func saveLog(path, kind string, log any, races int) error {
	if err := saveJSONAtomic(path, log); err != nil {
		applog.Error(kind+" log write failed", "component", "store", "file", path, "err", err)
		return err
	}
	applog.Info(kind+" log written", "component", "store", "file", path, "races", races)
	return nil
}

func stampEnvelope(e *Envelope, s persona.Session) {
	e.Version = SchemaVersion
	e.Role = s.Role
	e.Team = s.Team
	e.Sequence++
	e.WrittenAt = time.Now().UTC()
}
