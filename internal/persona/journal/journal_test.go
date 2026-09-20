package journal

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// newTestManager builds a Manager with injectable shared-write behavior and a
// negligible backoff, so retry tests do not spend real time waiting. The local
// write always succeeds (in-memory), matching the assumption the whole
// package rests on: local disk is always available.
func newTestManager(t *testing.T, sharedWrite func([]byte) error) *Manager {
	t.Helper()
	swapBackoff(t, time.Microsecond, 2*time.Microsecond)

	m := &Manager{
		sharedPath:  "shared.json",
		localPath:   "local.json",
		regattaKey:  "regatta-key",
		localWrite:  func([]byte) error { return nil },
		sharedWrite: sharedWrite,
		subs:        make(map[int]func(Status)),
		wake:        make(chan struct{}, 1),
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
		completedCh: make(chan struct{}),
		status:      Status{State: StateSynced, Since: time.Now()},
	}
	go m.run()
	t.Cleanup(m.Close)
	return m
}

func swapBackoff(t *testing.T, base, ceiling time.Duration) {
	t.Helper()
	origBase, origCeiling := backoffBase, backoffCap
	backoffBase, backoffCap = base, ceiling
	t.Cleanup(func() { backoffBase, backoffCap = origBase, origCeiling })
}

func swapFastPathWindow(t *testing.T, d time.Duration) {
	t.Helper()
	orig := fastPathWindow
	fastPathWindow = d
	t.Cleanup(func() { fastPathWindow = orig })
}

// waitForState blocks until m reaches want, or fails the test after timeout.
func waitForState(t *testing.T, m *Manager, want State, timeout time.Duration) Status {
	t.Helper()
	ch := make(chan Status, 16)
	unsub := m.Subscribe(func(s Status) {
		select {
		case ch <- s:
		default:
		}
	})
	defer unsub()

	deadline := time.After(timeout)
	for {
		select {
		case s := <-ch:
			if s.State == want {
				return s
			}
		case <-deadline:
			t.Fatalf("timed out waiting for state %q; last status %+v", want, m.Status())
			return Status{}
		}
	}
}

func TestWrite_LocalFailureReturnsErrorAndSetsStatus(t *testing.T) {
	localErr := errors.New("disk full")
	m := &Manager{
		sharedPath:  "shared.json",
		localPath:   "local.json",
		localWrite:  func([]byte) error { return localErr },
		sharedWrite: func([]byte) error { t.Fatal("shared write should never be attempted"); return nil },
		subs:        make(map[int]func(Status)),
		wake:        make(chan struct{}, 1),
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
		completedCh: make(chan struct{}),
		status:      Status{State: StateSynced, Since: time.Now()},
	}
	go m.run()
	t.Cleanup(m.Close)

	err := m.Write(map[string]int{"a": 1})
	if !errors.Is(err, localErr) {
		t.Fatalf("expected the local write error back, got %v", err)
	}
	if got := m.Status().State; got != StateLocalWriteFailed {
		t.Errorf("expected StateLocalWriteFailed, got %s", got)
	}
}

func TestWrite_SharedSucceeds_StatusEndsSynced(t *testing.T) {
	m := newTestManager(t, func([]byte) error { return nil })

	if err := m.Write(map[string]int{"a": 1}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	waitForState(t, m, StateSynced, time.Second)
}

func TestWrite_SharedFailsThenSucceeds_RetriesAndEventuallySyncs(t *testing.T) {
	var calls int
	var mu sync.Mutex
	m := newTestManager(t, func([]byte) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls < 3 {
			return errors.New("share unreachable")
		}
		return nil
	})

	// Subscribe before writing, and run Write in a goroutine (it now blocks
	// until its own attempt resolves): with the negligible backoff
	// newTestManager sets up, the fail-fail-succeed sequence can complete in
	// well under a millisecond, so subscribing only after Write returns risks
	// missing the transient Retrying state entirely. Subscribing first
	// guarantees every transition is observed regardless of how fast the
	// retry loop runs.
	seen := make(chan State, 16)
	unsub := m.Subscribe(func(s Status) {
		select {
		case seen <- s.State:
		default:
		}
	})
	defer unsub()

	writeDone := make(chan error, 1)
	go func() { writeDone <- m.Write(map[string]int{"a": 1}) }()

	sawRetrying := false
	deadline := time.After(time.Second)
waitLoop:
	for {
		select {
		case s := <-seen:
			if s == StateRetrying {
				sawRetrying = true
			}
			if s == StateSynced && sawRetrying {
				break waitLoop
			}
		case <-deadline:
			t.Fatalf("timed out waiting for a Retrying transition followed by Synced")
		}
	}

	if err := <-writeDone; err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Errorf("expected 3 attempts, got %d", calls)
	}
}

func TestWrite_NeverBlocksOnAnUnreachableSharedPath(t *testing.T) {
	m := newTestManager(t, func([]byte) error { return errors.New("share unreachable, forever") })

	done := make(chan error, 1)
	go func() { done <- m.Write(map[string]int{"a": 1}) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected Write to succeed locally even though the shared path is down, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Write blocked on an unreachable shared path")
	}

	waitForState(t, m, StateRetrying, time.Second)
}

func TestWrite_HungSharedWrite_ReturnsAfterFastPathWindow(t *testing.T) {
	swapFastPathWindow(t, 30*time.Millisecond)

	block := make(chan struct{}) // never closed while the attempt should still be hung
	m := newTestManager(t, func([]byte) error {
		<-block
		return nil
	})
	// Registered after newTestManager's own t.Cleanup(m.Close), so this runs
	// first (t.Cleanup is LIFO) and unblocks the hung attempt before Close
	// waits for the flusher goroutine to exit.
	t.Cleanup(func() { close(block) })

	start := time.Now()
	if err := m.Write("v1"); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("expected Write to give up waiting near fastPathWindow (30ms), took %v", elapsed)
	}

	// The attempt is still hung in the background; nothing has resolved yet.
	if got := m.Status().State; got != StateQueued {
		t.Errorf("expected StateQueued while the attempt is still hung, got %s", got)
	}
}

func TestWrite_NewerValueSupersedesOlderPendingDuringFlush(t *testing.T) {
	release := make(chan struct{})
	var attempted [][]byte
	var mu sync.Mutex

	m := newTestManager(t, func(b []byte) error {
		mu.Lock()
		attempted = append(attempted, append([]byte(nil), b...))
		first := len(attempted) == 1
		mu.Unlock()
		if first {
			<-release // hold the first attempt open until the test writes a newer value
		}
		return nil
	})

	// Both calls now block their own goroutine until their attempt resolves
	// (fastPathWindow), so v1 and v2 must run concurrently: v1's attempt is
	// held open by the mock above until release closes, and only then does
	// the flusher move on to attempt v2.
	v1Done := make(chan error, 1)
	go func() { v1Done <- m.Write("v1") }()
	time.Sleep(20 * time.Millisecond) // let the flusher pick up v1 and block inside sharedWrite

	v2Done := make(chan error, 1)
	go func() { v2Done <- m.Write("v2") }()
	time.Sleep(20 * time.Millisecond) // let v2 enqueue and start waiting on its own completion

	close(release)

	if err := <-v1Done; err != nil {
		t.Fatalf("Write v1 returned error: %v", err)
	}
	if err := <-v2Done; err != nil {
		t.Fatalf("Write v2 returned error: %v", err)
	}

	waitForState(t, m, StateSynced, time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(attempted) < 2 {
		t.Fatalf("expected at least 2 shared-write attempts (v1 then v2), got %d", len(attempted))
	}
	last := string(attempted[len(attempted)-1])
	if last != `"v2"` {
		t.Errorf("expected the last shared write to carry v2, got %s", last)
	}
}

func TestSubscribe_DeliversCurrentStatusImmediately(t *testing.T) {
	m := newTestManager(t, func([]byte) error { return nil })

	var got Status
	unsub := m.Subscribe(func(s Status) { got = s })
	defer unsub()

	if got.State != StateSynced {
		t.Errorf("expected the initial subscription callback to report Synced, got %s", got.State)
	}
}

func TestSubscribe_UnsubscribeStopsDelivery(t *testing.T) {
	m := newTestManager(t, func([]byte) error { return errors.New("down") })

	var mu sync.Mutex
	var count int
	unsub := m.Subscribe(func(Status) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	unsub()

	if err := m.Write("v1"); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Errorf("expected only the immediate delivery on Subscribe (count=1), got %d", count)
	}
}

func TestNudge_SkipsRemainingBackoff(t *testing.T) {
	swapBackoff(t, time.Hour, time.Hour) // would never retry in time without Nudge
	var calls int
	var mu sync.Mutex
	m := &Manager{
		sharedPath: "shared.json",
		localPath:  "local.json",
		localWrite: func([]byte) error { return nil },
		sharedWrite: func([]byte) error {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls < 2 {
				return errors.New("down")
			}
			return nil
		},
		subs:        make(map[int]func(Status)),
		wake:        make(chan struct{}, 1),
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
		completedCh: make(chan struct{}),
		status:      Status{State: StateSynced, Since: time.Now()},
	}
	go m.run()
	t.Cleanup(m.Close)

	if err := m.Write("v1"); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	waitForState(t, m, StateRetrying, time.Second)

	m.Nudge()
	waitForState(t, m, StateSynced, time.Second)
}

func TestClose_StopsTheFlusherGoroutine(t *testing.T) {
	m := newTestManager(t, func([]byte) error { return nil })
	m.Close()

	select {
	case <-m.done:
	default:
		t.Error("expected done to be closed after Close")
	}
}
