package timesync

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

const stubRTT = 10 * time.Millisecond

// stubQuery returns a QueryFunc backed by fixed per-host offsets. A host listed
// in fail returns an error; a host with no entry anywhere also errors.
func stubQuery(offsets map[string]time.Duration, fail ...string) func(string, time.Duration) (time.Duration, time.Duration, error) {
	failed := make(map[string]bool, len(fail))
	for _, h := range fail {
		failed[h] = true
	}
	return func(host string, _ time.Duration) (time.Duration, time.Duration, error) {
		if failed[host] {
			return 0, 0, errors.New("unreachable")
		}
		off, ok := offsets[host]
		if !ok {
			return 0, 0, errors.New("no stub for " + host)
		}
		return off, stubRTT, nil
	}
}

func waitFor(t *testing.T, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestNewAppliesDefaults(t *testing.T) {
	s := New(Config{})

	if !slices.Equal(s.cfg.Servers, DefaultServers) {
		t.Fatalf("servers = %v, want %v", s.cfg.Servers, DefaultServers)
	}
	if &s.cfg.Servers[0] == &DefaultServers[0] {
		t.Fatal("cfg.Servers aliases the package DefaultServers slice")
	}
	if s.cfg.Interval != DefaultInterval || s.cfg.Timeout != DefaultTimeout {
		t.Fatalf("interval/timeout = %v/%v", s.cfg.Interval, s.cfg.Timeout)
	}
	if s.cfg.QueryFunc == nil {
		t.Fatal("QueryFunc not defaulted")
	}
	if got := s.Ref(); got.Source != SourcePending {
		t.Fatalf("initial source = %q, want %q", got.Source, SourcePending)
	}
}

func TestMeasureOnceMedianOfThree(t *testing.T) {
	s := New(Config{
		Servers: []string{"x", "y", "z"},
		QueryFunc: stubQuery(map[string]time.Duration{
			"x": 100 * time.Millisecond,
			"y": 5 * time.Second, // outlier
			"z": 120 * time.Millisecond,
		}),
	})

	s.measureOnce(context.Background())

	ref := s.Ref()
	if ref.Offset != 120*time.Millisecond {
		t.Fatalf("offset = %v, want 120ms (median)", ref.Offset)
	}
	if ref.Source != "ntp:z" {
		t.Fatalf("source = %q, want ntp:z", ref.Source)
	}
	if ref.RTT != stubRTT {
		t.Fatalf("rtt = %v, want %v", ref.RTT, stubRTT)
	}
	if ref.MeasuredAt.IsZero() {
		t.Fatal("MeasuredAt not set")
	}
}

func TestMeasureOnceMedianOfTwoPicksUpper(t *testing.T) {
	s := New(Config{
		Servers: []string{"a", "b"},
		QueryFunc: stubQuery(map[string]time.Duration{
			"a": 180 * time.Millisecond,
			"b": 200 * time.Millisecond,
		}),
	})

	s.measureOnce(context.Background())

	if ref := s.Ref(); ref.Offset != 200*time.Millisecond || ref.Source != "ntp:b" {
		t.Fatalf("ref = %+v, want offset 200ms from ntp:b", ref)
	}
}

func TestMeasureOnceSkipsFailedServers(t *testing.T) {
	s := New(Config{
		Servers:   []string{"dead", "live"},
		QueryFunc: stubQuery(map[string]time.Duration{"live": 50 * time.Millisecond}, "dead"),
	})

	s.measureOnce(context.Background())

	if ref := s.Ref(); ref.Offset != 50*time.Millisecond || ref.Source != "ntp:live" {
		t.Fatalf("ref = %+v, want offset 50ms from ntp:live", ref)
	}
}

func TestMeasureOnceAllServersFail(t *testing.T) {
	s := New(Config{
		Servers:   []string{"a", "b", "c"},
		QueryFunc: stubQuery(nil, "a", "b", "c"),
	})

	s.measureOnce(context.Background())

	ref := s.Ref()
	if ref.Source != SourceNone {
		t.Fatalf("source = %q, want %q", ref.Source, SourceNone)
	}
	if ref.Offset != 0 {
		t.Fatalf("offset = %v, want 0 when unreachable", ref.Offset)
	}
	if ref.MeasuredAt.IsZero() {
		t.Fatal("MeasuredAt should still be set on a failed measurement")
	}
}

func TestNowAppliesOffset(t *testing.T) {
	s := New(Config{
		Servers:   []string{"a"},
		QueryFunc: stubQuery(map[string]time.Duration{"a": 2 * time.Second}),
	})
	s.measureOnce(context.Background())

	before := time.Now().Add(2 * time.Second)
	now, ref := s.Now()
	after := time.Now().Add(2 * time.Second)

	if ref.Offset != 2*time.Second {
		t.Fatalf("ref.Offset = %v, want 2s", ref.Offset)
	}
	if now.Before(before.Add(-50*time.Millisecond)) || now.After(after.Add(50*time.Millisecond)) {
		t.Fatalf("Now() = %v, expected ~now+2s", now)
	}
}

func TestNowBeforeFirstMeasurement(t *testing.T) {
	s := New(Config{Servers: []string{"a"}, QueryFunc: stubQuery(map[string]time.Duration{"a": time.Hour})})

	now, ref := s.Now()

	if ref.Source != SourcePending || ref.Offset != 0 {
		t.Fatalf("ref = %+v, want pending/0 before measurement", ref)
	}
	if d := time.Since(now); d < -time.Second || d > time.Second {
		t.Fatalf("Now() = %v, expected ~time.Now()", now)
	}
}

func TestNegativeOffsetMedian(t *testing.T) {
	s := New(Config{
		Servers: []string{"a", "b", "c"},
		QueryFunc: stubQuery(map[string]time.Duration{
			"a": -300 * time.Millisecond,
			"b": -280 * time.Millisecond,
			"c": 4 * time.Second,
		}),
	})

	s.measureOnce(context.Background())

	if ref := s.Ref(); ref.Offset != -280*time.Millisecond || ref.Source != "ntp:b" {
		t.Fatalf("ref = %+v, want -280ms from ntp:b", ref)
	}
}

func TestStartRequeriesUntilStopped(t *testing.T) {
	calls := make(chan string, 16)
	s := New(Config{
		Servers:  []string{"a"},
		Interval: 15 * time.Millisecond,
		QueryFunc: func(host string, _ time.Duration) (time.Duration, time.Duration, error) {
			calls <- host
			return 5 * time.Millisecond, stubRTT, nil
		},
	})

	s.Start(context.Background())

	// Initial measurement plus at least two ticks.
	for i := 0; i < 3; i++ {
		select {
		case <-calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected re-query #%d, timed out", i+1)
		}
	}

	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return")
	}

	// Drain, then confirm no further calls arrive after Stop.
	for len(calls) > 0 {
		<-calls
	}
	select {
	case <-calls:
		t.Fatal("query ran after Stop")
	case <-time.After(60 * time.Millisecond):
	}
}

func TestStopSafeWithoutStart(t *testing.T) {
	New(Config{}).Stop() // must not panic or block
}

func TestStartIsIdempotent(t *testing.T) {
	var started atomic.Int32
	block := make(chan struct{})
	s := New(Config{
		Servers:  []string{"a"},
		Interval: time.Hour,
		QueryFunc: func(string, time.Duration) (time.Duration, time.Duration, error) {
			started.Add(1)
			<-block
			return 0, 0, errors.New("done")
		},
	})

	s.Start(context.Background())
	s.Start(context.Background()) // second call is a no-op

	waitFor(t, time.Second, func() bool { return started.Load() >= 1 })
	time.Sleep(20 * time.Millisecond)
	if got := started.Load(); got != 1 {
		t.Fatalf("query goroutine ran %d times, want 1 (second Start started another loop)", got)
	}

	close(block)
	s.Stop()
}

func TestPackageSingletonStartNowStop(t *testing.T) {
	prev := current()
	t.Cleanup(func() {
		Stop()
		stdMu.Lock()
		std = prev
		stdMu.Unlock()
	})

	Start(context.Background(), Config{
		Servers:   []string{"lan"},
		Interval:  time.Hour,
		QueryFunc: stubQuery(map[string]time.Duration{"lan": 750 * time.Millisecond}),
	})

	waitFor(t, 2*time.Second, func() bool { return Ref().Source == "ntp:lan" })

	now, ref := Now()
	if ref.Offset != 750*time.Millisecond {
		t.Fatalf("ref.Offset = %v, want 750ms", ref.Offset)
	}
	if d := time.Until(now); d < 650*time.Millisecond || d > 850*time.Millisecond {
		t.Fatalf("Now() offset ~%v from real now, want ~750ms", d)
	}

	Stop()
}
