// Package journal durably stages a persona's own timing writes to local disk
// before writing them to the shared regattaData path, and retries the shared
// write with backoff when that path is unreachable - an SMB outage or a
// cloud-sync stall never blocks collecting a value, only delays it reaching
// other personas (docs/features/personas/persona-plan.md section 13,
// docs/features/personas/shared-storage-options.md sections 5 and 8).
//
// StartLog/FinishLog are always saved as the complete document, never a delta
// (persona-plan.md section 5b), and only one persona ever writes a given file
// (store.ErrWrongPersona). Those two facts are what keep this package small: a
// Manager only ever needs to hold the single latest pending value for its
// file, never merge concurrent writers or replay a sequence of deltas.
//
// The local staging directory must never live under a persona.Session's Root
// (regattaData/) - if Root is itself the unreachable share, journaling onto it
// defeats the point - and must never be registered with internal/watcher.
package journal

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/comagnaw/regattaClock/internal/applog"
)

// backoffBase and backoffCap shape the shared-write retry loop: the same
// doubling-backoff shape as internal/regattacentral's HTTP retry
// (retryBaseBackoff), capped because an SMB outage or cloud stall can last
// minutes, not milliseconds - retrying indefinitely at a bounded interval is
// the point, not giving up. Vars rather than consts so tests can shrink them
// (see swapBackoff in journal_test.go), mirroring
// internal/filesystem's renameBaseBackoff seam.
var (
	backoffBase = 500 * time.Millisecond
	backoffCap  = 30 * time.Second
)

// fastPathWindow bounds how long Write waits for the shared-write attempt it
// just queued to resolve before giving up and returning anyway. Against any
// healthy target - every existing SaveStart/SaveFinish test, and normal
// operation - the attempt resolves in milliseconds and Write returns only
// once the shared file actually holds the new value, exactly like the
// pre-journal synchronous write did. Only a target that is genuinely stuck
// (not just erroring - an unreachable share simply fails its open/write fast)
// makes Write stop waiting early; the attempt keeps running in the
// background regardless. A var, not a const, so a test can shrink it to
// exercise that "stop waiting early" path without spending real time on it.
var fastPathWindow = 2 * time.Second

// State is where a Manager's latest write stands relative to the shared path.
type State string

const (
	// StateSynced means the shared path holds the latest value written.
	StateSynced State = "synced"
	// StateQueued means a value is durably staged locally and waiting for its
	// first shared-write attempt.
	StateQueued State = "queued"
	// StateRetrying means at least one shared-write attempt has failed and the
	// flusher is backing off before trying again.
	StateRetrying State = "retrying"
	// StateLocalWriteFailed means the local durable write itself failed -
	// rarer and more serious than an unreachable shared path, since local
	// disk is what the rest of this package assumes is always available.
	StateLocalWriteFailed State = "local_write_failed"
)

// Status is a snapshot of a Manager's sync state, for a UI banner.
type Status struct {
	State   State
	Since   time.Time
	LastErr error
}

// Manager durably stages the single file a persona.Session owns (its
// WritePath) to local disk before writing it to the shared path, and retries
// the shared write in the background when the shared path is not reachable.
// Construct one with For, never directly.
type Manager struct {
	sharedPath string
	localPath  string
	regattaKey string

	// localWrite and sharedWrite perform the two atomic writes; both default
	// to writeAtomic (registry.go) and are indirection seams so tests can
	// simulate an unreachable shared path without a real filesystem outage -
	// the same shape as internal/filesystem's renameFunc/retryableRename seam.
	localWrite  func([]byte) error
	sharedWrite func([]byte) error

	mu             sync.Mutex
	pending        []byte // nil once the shared path has caught up
	pendingVersion int64
	// completedVersion and completedCh let Write wait for its own
	// pendingVersion to be attempted (successfully or not) without caring
	// which: completedCh is closed and replaced every time run finishes an
	// attempt, after advancing completedVersion to that attempt's version.
	completedVersion int64
	completedCh      chan struct{}
	recovered        []byte // set once by recoverPending; consumed by Recovered
	status           Status
	subs             map[int]func(Status)
	nextSub          int

	wake      chan struct{}
	quit      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// Write durably stages v to local disk (always, synchronously - this is the
// step the "timer click path is sacred" principle depends on, since local
// disk is assumed always available), then hands the shared write to the
// background flusher and waits up to fastPathWindow for that attempt to
// resolve. It returns an error only when the local write itself failed - not
// when the shared write fails or times out waiting. Against a healthy
// target, Write does not return until the shared file actually holds the new
// value; against an unreachable or hung one, it stops waiting and lets the
// flusher keep retrying, surfaced via Status/Subscribe instead of a
// synchronous error.
func (m *Manager) Write(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("journal: could not marshal write for %s: %w", m.sharedPath, err)
	}

	if err := m.localWrite(b); err != nil {
		m.setStatus(Status{State: StateLocalWriteFailed, Since: time.Now(), LastErr: err})
		return fmt.Errorf("journal: local write for %s failed: %w", m.sharedPath, err)
	}

	m.mu.Lock()
	m.pending = b
	m.pendingVersion++
	v64 := m.pendingVersion
	ch := m.completedCh
	// Since marks when this file first fell out of sync, not when this
	// particular Write happened: preserved across a Queued/Retrying episode
	// so a status banner can show "unreachable for 45s" instead of resetting
	// every time a newer value supersedes an older, still-unflushed one.
	since := m.status.Since
	if m.status.State == StateSynced || since.IsZero() {
		since = time.Now()
	}
	m.mu.Unlock()
	m.setStatus(Status{State: StateQueued, Since: since})
	m.poke()

	deadline := time.After(fastPathWindow)
	for {
		select {
		case <-ch:
			m.mu.Lock()
			done := m.completedVersion >= v64
			ch = m.completedCh
			m.mu.Unlock()
			if done {
				return nil
			}
			// A different, unrelated attempt (from before this Write even
			// enqueued) resolved and rotated the channel; keep waiting on the
			// fresh one for our own version.
		case <-deadline:
			return nil
		}
	}
}

// Status returns the current sync state.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Subscribe registers fn to be called with every subsequent Status
// transition, and once immediately with the current status. The returned func
// removes the subscription; callers (a UI banner) must call it when they stop
// watching.
func (m *Manager) Subscribe(fn func(Status)) (unsubscribe func()) {
	m.mu.Lock()
	id := m.nextSub
	m.nextSub++
	m.subs[id] = fn
	current := m.status
	m.mu.Unlock()

	fn(current)
	return func() {
		m.mu.Lock()
		delete(m.subs, id)
		m.mu.Unlock()
	}
}

// Nudge retries the shared write immediately instead of waiting out the
// flusher's current backoff. No-op if nothing is pending. Wired to a banner's
// "Retry Now" action.
func (m *Manager) Nudge() {
	m.poke()
}

// Close stops the background flusher and waits for it to exit. It does not
// wait for a pending write to succeed - closing a Manager whose shared path
// is down simply leaves the pending value durably queued for the next launch
// (see recoverPending).
func (m *Manager) Close() {
	m.closeOnce.Do(func() { close(m.quit) })
	<-m.done
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.status = s
	subs := make([]func(Status), 0, len(m.subs))
	for _, fn := range m.subs {
		subs = append(subs, fn)
	}
	m.mu.Unlock()

	for _, fn := range subs {
		fn(s)
	}
}

func (m *Manager) poke() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// run is the single background flusher goroutine for this Manager: one
// attempt at a time, so there is never a race between a "fast path" write and
// a retry - Write and run share the same pending slot and the same code path.
func (m *Manager) run() {
	defer close(m.done)
	backoff := backoffBase

	for {
		select {
		case <-m.quit:
			return
		case <-m.wake:
		}

		for {
			m.mu.Lock()
			b := m.pending
			v := m.pendingVersion
			m.mu.Unlock()
			if b == nil {
				break
			}

			err := m.sharedWrite(b)

			m.mu.Lock()
			caughtUp := err == nil && m.pendingVersion == v
			if caughtUp {
				m.pending = nil
			}
			if v > m.completedVersion {
				m.completedVersion = v
			}
			doneCh := m.completedCh
			m.completedCh = make(chan struct{})
			m.mu.Unlock()
			close(doneCh)

			if err == nil {
				backoff = backoffBase
				if caughtUp {
					m.setStatus(Status{State: StateSynced, Since: time.Now()})
					break
				}
				continue // a newer value landed mid-flush; attempt it right away
			}

			// Since carries forward from the Queued transition that started
			// this episode (or an earlier Retrying attempt within it) rather
			// than resetting on every attempt - see the comment in Write.
			m.mu.Lock()
			since := m.status.Since
			m.mu.Unlock()
			m.setStatus(Status{State: StateRetrying, Since: since, LastErr: err})
			applog.Warn("journal: shared write failed, retrying", "component", "journal",
				"file", m.sharedPath, "err", err, "wait_ms", backoff.Milliseconds())

			select {
			case <-m.quit:
				return
			case <-m.wake:
				backoff = backoffBase // fresh data or a manual Nudge deserves an immediate attempt
			case <-time.After(backoff):
				backoff = min(backoff*2, backoffCap)
			}
		}
	}
}
