// Package persona defines the fixed set of operator roles that share one
// regattaData directory, and where on disk each one reads and writes.
//
// It is a leaf: the path helpers return plain strings and the package performs
// no I/O. Loading and saving the files these paths point at belong in
// internal/persona/store (phase 3), which imports this package for the Role and
// Team types. persona must never import store back.
package persona

import (
	"path/filepath"
	"strings"
)

// Role is what a persona does with the race clock.
type Role string

// Team keys a set of timing files. Primary and secondary are independent
// ST/FT pairings for the same regatta; executive is the director and any
// future non-timing officials.
type Team string

const (
	RoleDirector Role = "director"
	RoleStart    Role = "start"
	RoleFinish   Role = "finish"

	TeamExecutive Team = "executive"
	TeamPrimary   Team = "primary"
	TeamSecondary Team = "secondary"
)

// Layout of the shared regattaData tree. These live here rather than in
// internal/common because persona owns the multi-writer directory layout;
// common.LogsDir is the one name shared with the pre-persona code.
const (
	dirDirector  = "director"
	dirTiming    = "timing"
	fileSchedule = "regattaSchedule.json"
	fileStart    = "start.json"
	fileFinish   = "finish.json"
)

// Definition is one persona. New personas are added by appending a struct
// literal to Registry; nothing else has to change.
type Definition struct {
	ID        string // stable short code: "pst", "sst", "pft", "sft", "rd"; the persona_id log attr
	Role      Role
	Team      Team
	Label     string // human label for the picker, e.g. "Primary Start Timer"
	Challenge string // code the operator types to select this persona
	File      string // owned timing file: "start.json" / "finish.json"; empty for the director
}

// Registry is the personas offered on the timer startup screen, in display
// order. The director is not here; it has its own entry point.
var Registry = []Definition{
	{ID: "pst", Role: RoleStart, Team: TeamPrimary, Label: "Primary Start Timer", Challenge: "rc-pst", File: fileStart},
	{ID: "sst", Role: RoleStart, Team: TeamSecondary, Label: "Secondary Start Timer", Challenge: "rc-sst", File: fileStart},
	{ID: "pft", Role: RoleFinish, Team: TeamPrimary, Label: "Primary Finish Timer", Challenge: "rc-pft", File: fileFinish},
	{ID: "sft", Role: RoleFinish, Team: TeamSecondary, Label: "Secondary Finish Timer", Challenge: "rc-sft", File: fileFinish},
}

// DirectorDefinition is used by the separate director binary and is not offered
// on the timer picker. The director owns regattaSchedule.json, not a timing
// file, so File is empty.
var DirectorDefinition = Definition{
	ID: "rd", Role: RoleDirector, Team: TeamExecutive, Label: "Regatta Director", Challenge: "rc-rd", File: "",
}

// All returns every persona - the timer registry plus the director - as a fresh
// slice the caller may reorder or filter without affecting the package state.
func All() []Definition {
	out := make([]Definition, 0, len(Registry)+1)
	out = append(out, Registry...)
	out = append(out, DirectorDefinition)
	return out
}

// ByID looks up a persona by its stable code (as stored in a log line or a file
// envelope). The second result is false if no persona has that ID.
func ByID(id string) (Definition, bool) {
	for _, d := range All() {
		if d.ID == id {
			return d, true
		}
	}
	return Definition{}, false
}

// MatchesChallenge reports whether input is this persona's challenge code,
// ignoring surrounding whitespace and letter case.
func (d Definition) MatchesChallenge(input string) bool {
	got := normalizeChallenge(input)
	return got != "" && got == normalizeChallenge(d.Challenge)
}

func normalizeChallenge(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Session is a chosen persona bound to a regattaData directory. It is created
// once at startup and threaded through Regatta and Clock rather than consulted
// from a global.
type Session struct {
	Definition
	Root string // absolute path to the chosen regattaData directory
}

// SchedulePathIn is <root>/director/regattaSchedule.json. Exposed so callers
// that only have a candidate root - directory validation before a Session
// exists - can locate the schedule without reconstructing the layout.
func SchedulePathIn(root string) string {
	return filepath.Join(root, dirDirector, fileSchedule)
}

// SchedulePath is regattaData/director/regattaSchedule.json - the schedule
// every persona reads and only the director writes.
func (s Session) SchedulePath() string {
	return SchedulePathIn(s.Root)
}

// StartPath is this session's team's start.json. The start timer owns it; the
// finish timer reads it for each race's start time.
func (s Session) StartPath() string {
	return filepath.Join(s.timingDir(), fileStart)
}

// FinishPath is this session's team's finish.json. The finish timer owns it; it
// is the race-results file the director reconciles.
func (s Session) FinishPath() string {
	return filepath.Join(s.timingDir(), fileFinish)
}

// WritePath is the single file this persona is allowed to write: its team's
// start.json for a start timer, its team's finish.json for a finish timer, and
// the schedule for the director.
func (s Session) WritePath() string {
	if s.Role == RoleDirector {
		return s.SchedulePath()
	}
	return filepath.Join(s.timingDir(), s.File)
}

func (s Session) timingDir() string {
	return filepath.Join(s.Root, dirTiming, string(s.Team))
}
