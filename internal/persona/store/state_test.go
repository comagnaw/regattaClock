package store

import (
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/persona"
)

func TestDeriveTeamState(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name  string
		start StartRecord
		res   RaceResult
		want  TeamState
	}{
		{
			name:  "nothing recorded",
			start: StartRecord{},
			res:   RaceResult{},
			want:  StateNotStarted,
		},
		{
			name:  "start recorded then cleared, no restart yet",
			start: StartRecord{Cleared: []ClearedStart{{}}},
			res:   RaceResult{},
			want:  StateStopped,
		},
		{
			name:  "start recorded, FT has not started",
			start: StartRecord{StartedAt: ptrTime(now)},
			res:   RaceResult{},
			want:  StateStartRecorded,
		},
		{
			name:  "FT clock running, no stop, no winning time, not approved",
			start: StartRecord{StartedAt: ptrTime(now)},
			res:   RaceResult{FirstFinishAt: ptrTime(now)},
			want:  StateTimingInProgress,
		},
		{
			name:  "primary FT clicked Stop, awaiting approval",
			start: StartRecord{StartedAt: ptrTime(now)},
			res:   RaceResult{FirstFinishAt: ptrTime(now), StoppedAt: ptrTime(now)},
			want:  StatePendingApproval,
		},
		{
			name:  "secondary FT saved (Save and Close), no Stop signal ever set",
			start: StartRecord{StartedAt: ptrTime(now)},
			res:   RaceResult{FirstFinishAt: ptrTime(now), WinningTime: "06:00.0"},
			want:  StateSaved,
		},
		{
			name:  "primary FT approved",
			start: StartRecord{StartedAt: ptrTime(now)},
			res:   RaceResult{FirstFinishAt: ptrTime(now), StoppedAt: ptrTime(now), WinningTime: "06:00.0", Approved: true},
			want:  StateApproved,
		},
		{
			name:  "approved wins over every other signal",
			start: StartRecord{},
			res:   RaceResult{Approved: true},
			want:  StateApproved,
		},
		{
			name:  "cleared start history is irrelevant once the FT has started",
			start: StartRecord{StartedAt: ptrTime(now), Cleared: []ClearedStart{{}}},
			res:   RaceResult{FirstFinishAt: ptrTime(now)},
			want:  StateTimingInProgress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveTeamState(tt.start, tt.res); got != tt.want {
				t.Errorf("DeriveTeamState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTeamState_DisplayText(t *testing.T) {
	tests := []struct {
		name      string
		state     TeamState
		primary   string
		secondary string
	}{
		{"not started", StateNotStarted, "Pending Start", "Pending Start"},
		{"stopped (ST-side halt)", StateStopped, "Stopped", "Stopped"},
		{"start recorded", StateStartRecorded, "On the Water", "On the Water"},
		{"timing in progress", StateTimingInProgress, "On the Water", "On the Water"},
		{"pending approval", StatePendingApproval, "Pending Approval", "Pending Approval"},
		{"saved", StateSaved, "Saved", "Saved"},
		{"approved", StateApproved, "Official", "Official"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/primary", func(t *testing.T) {
			if got := tt.state.DisplayText(persona.TeamPrimary); got != tt.primary {
				t.Errorf("DisplayText(primary) = %q, want %q", got, tt.primary)
			}
		})
		t.Run(tt.name+"/secondary", func(t *testing.T) {
			if got := tt.state.DisplayText(persona.TeamSecondary); got != tt.secondary {
				t.Errorf("DisplayText(secondary) = %q, want %q", got, tt.secondary)
			}
		})
	}
}

func TestCanPublish(t *testing.T) {
	if CanPublish(RaceResult{}) {
		t.Error("CanPublish() = true for an unapproved result, want false")
	}
	if !CanPublish(RaceResult{Approved: true}) {
		t.Error("CanPublish() = false for an approved result, want true")
	}
}

func TestCanTrackWallClock(t *testing.T) {
	if CanTrackWallClock(StartRecord{}) {
		t.Error("CanTrackWallClock() = true with no StartedAt, want false")
	}
	now := time.Now().UTC()
	if !CanTrackWallClock(StartRecord{StartedAt: &now}) {
		t.Error("CanTrackWallClock() = false with StartedAt set, want true")
	}
}
