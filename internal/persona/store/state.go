package store

import "github.com/comagnaw/regattaClock/internal/persona"

// TeamState is one team's canonical race-timing state, derived - never
// stored - from a StartRecord and a RaceResult (race-state-machine.md). The
// same team can independently reach NotStarted/Stopped/StartRecorded from
// the ST side while the FT side reaches TimingInProgress/PendingApproval/
// Saved/Approved; DeriveTeamState folds both into one value so every
// consumer (the FT's own clock, the race tree, the RD tree) reads one
// canonical state instead of re-deriving it.
type TeamState int

const (
	// StateNotStarted - no start time recorded, and no cleared-start history.
	StateNotStarted TeamState = iota

	// StateStopped - ST-side: a start was recorded, then cleared - an
	// on-water halt (the referees stopping the race), not a routine
	// data-entry correction. Inferred from StartRecord.Cleared, not a
	// dedicated field.
	StateStopped

	// StateStartRecorded - the ST has recorded a start time, but the FT has
	// not yet clicked clock Start for this race.
	StateStartRecorded

	// StateTimingInProgress - the FT's clock is running (or stopped but not
	// yet reviewed - see StatePendingApproval) for this race.
	StateTimingInProgress

	// StatePendingApproval - primary team only. The FT clicked Stop and is
	// done collecting times; the race awaits Referee Approval. Unreachable
	// for the secondary team, which has no approval gate to await.
	StatePendingApproval

	// StateSaved - secondary team only. Save and Close has been clicked;
	// this is the secondary team's terminal state. Unreachable for the
	// primary team, whose only commit path is Referee Approval.
	StateSaved

	// StateApproved - primary team only. Referee Approval has been clicked.
	// Unreachable for the secondary team.
	StateApproved
)

// DeriveTeamState computes one team's canonical state from its own
// StartRecord and RaceResult. See race-state-machine.md's "Phase 1" for the
// full per-team milestone ladder this implements.
func DeriveTeamState(start StartRecord, res RaceResult) TeamState {
	switch {
	case res.Approved:
		return StateApproved
	case res.WinningTime != "":
		return StateSaved
	case res.StoppedAt != nil:
		return StatePendingApproval
	case res.FirstFinishAt != nil:
		return StateTimingInProgress
	case start.StartedAt != nil:
		return StateStartRecorded
	case len(start.Cleared) > 0:
		return StateStopped
	default:
		return StateNotStarted
	}
}

// DisplayText renders the "Display vocabulary" table from
// race-state-machine.md. team is always the team WHOSE state this is (the
// race's timing team - Primary or Secondary), never the viewer's own team; a
// caller reading primary-team data (e.g. the RD tree) always passes
// persona.TeamPrimary, regardless of its own session team.
func (s TeamState) DisplayText(team persona.Team) string {
	switch s {
	case StateNotStarted:
		return "Pending Start"
	case StateStopped:
		return "Stopped"
	case StateStartRecorded, StateTimingInProgress:
		return "On the Water"
	case StatePendingApproval:
		return "Pending Approval"
	case StateSaved:
		return "Saved"
	case StateApproved:
		return "Official"
	default:
		return "Pending Start"
	}
}

// CanPublish reports whether a race's result is official and ready for a
// downstream consumer (a race-tree "View Results"/"Publish" button, or a
// future persona's own gate) to act on.
func CanPublish(res RaceResult) bool {
	return res.Approved
}

// CanTrackWallClock reports whether a race has a recorded start time, the
// gate for a wall-clock display keyed off the same race (e.g. Streamer's
// "run clock" button - a separate, independent gate from CanPublish).
func CanTrackWallClock(start StartRecord) bool {
	return start.StartedAt != nil
}
