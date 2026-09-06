package timesync

import "time"

// Source values for ClockRef.Source that are not an "ntp:<host>" string.
const (
	// SourcePending is the source before the first measurement completes.
	SourcePending = "pending"

	// SourceNone is the source when every configured server was unreachable.
	// The offset is zero: readers can still compare wall clocks to detect
	// skew, they just cannot correct for it.
	SourceNone = "none"
)

// ClockRef records what timesync knew about this machine's clock offset at a
// single moment. It is stored alongside every captured timestamp rather than
// applied to it, so a measurement later found to be wrong can be corrected
// after the fact instead of being baked in.
//
// The persona store (phase 3) embeds this type on its envelope and per-record
// structures; it is defined here because timesync is what produces it.
type ClockRef struct {
	// Offset is added to a local timestamp to get the true time. Zero when
	// Source is "none" or "pending".
	Offset time.Duration

	// RTT is the round trip of the NTP query behind this offset, a rough
	// confidence bound on Offset.
	RTT time.Duration

	// Source is "ntp:<host>" for the server whose sample was used, "none" when
	// all servers failed, or "pending" before the first measurement.
	Source string

	// MeasuredAt is this machine's local wall clock when the offset was
	// measured.
	MeasuredAt time.Time
}

// Corrected returns t shifted by the offset. For a "none"/"pending" ref this is
// t unchanged.
func (c ClockRef) Corrected(t time.Time) time.Time { return t.Add(c.Offset) }
