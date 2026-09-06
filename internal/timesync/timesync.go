// Package timesync measures this machine's clock offset from NTP without ever
// setting the system clock. The winning time of a race is the finish timer's
// Start click minus the start timer's Start time, captured on two different
// laptops; if their clocks disagree by seconds every race time is silently
// wrong by that much. Recording each machine's offset lets the cross-machine
// subtraction be corrected at read time (trueTime = localTime + offset).
//
// Measurement runs on a background goroutine and is queried through a cached
// ClockRef, so a timing button click never waits on the network.
package timesync

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/comagnaw/regattaClock/internal/applog"
)

// DefaultServers are queried when Config.Servers is empty. Three lets the
// median survive one bad responder. Under smb mode phase 4b seeds the LAN NTP
// host ahead of these.
var DefaultServers = []string{"time.cloudflare.com", "time.google.com", "pool.ntp.org"}

const (
	// DefaultInterval is how often the offset is re-measured. A laptop waking
	// from sleep can jump, so this is not a one-shot at startup.
	DefaultInterval = 15 * time.Minute

	// DefaultTimeout bounds a single server query.
	DefaultTimeout = 5 * time.Second

	// SkewWarnThreshold is the |offset| above which a measurement is logged at
	// WARN rather than INFO.
	SkewWarnThreshold = time.Second
)

// Config configures a Syncer. The zero value is valid: every field falls back
// to its default.
type Config struct {
	// Servers to query, in preference order. Empty uses DefaultServers.
	Servers []string

	// Interval between re-measurements. <= 0 uses DefaultInterval.
	Interval time.Duration

	// Timeout for a single server query. <= 0 uses DefaultTimeout.
	Timeout time.Duration

	// QueryFunc measures the offset and round-trip time against one server.
	// nil uses the real SNTP query. Tests inject a stub.
	QueryFunc func(host string, timeout time.Duration) (offset, rtt time.Duration, err error)
}

func (c *Config) applyDefaults() {
	if len(c.Servers) == 0 {
		c.Servers = append([]string(nil), DefaultServers...)
	}
	if c.Interval <= 0 {
		c.Interval = DefaultInterval
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.QueryFunc == nil {
		c.QueryFunc = queryNTP
	}
}

// Syncer holds the most recent ClockRef and, once started, refreshes it on a
// ticker until its context is cancelled.
type Syncer struct {
	cfg Config

	mu     sync.RWMutex
	ref    ClockRef
	cancel context.CancelFunc
	done   chan struct{}
}

// New returns an unstarted Syncer. Its Ref reports SourcePending until the
// first measurement lands.
func New(cfg Config) *Syncer {
	cfg.applyDefaults()
	return &Syncer{cfg: cfg, ref: ClockRef{Source: SourcePending}}
}

// Start measures once immediately, then every cfg.Interval, until ctx is
// cancelled or Stop is called. A second call is a no-op.
func (s *Syncer) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.run(runCtx)
}

// Stop cancels the background loop and waits for it to exit. The loop checks
// for cancellation between server queries, so this returns within at most one
// server timeout. Safe to call without Start and safe to call more than once.
func (s *Syncer) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// Ref returns the most recent ClockRef.
func (s *Syncer) Ref() ClockRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ref
}

// Now returns the offset-corrected time and the ClockRef that produced it, so a
// caller can store the raw local time and the offset separately.
func (s *Syncer) Now() (time.Time, ClockRef) {
	ref := s.Ref()
	return ref.Corrected(time.Now()), ref
}

func (s *Syncer) run(ctx context.Context) {
	defer close(s.done)

	s.measureOnce(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.measureOnce(ctx)
		}
	}
}

func (s *Syncer) measureOnce(ctx context.Context) {
	type sample struct {
		host   string
		offset time.Duration
		rtt    time.Duration
	}

	var samples []sample
	for _, host := range s.cfg.Servers {
		select {
		case <-ctx.Done():
			return
		default:
		}

		offset, rtt, err := s.cfg.QueryFunc(host, s.cfg.Timeout)
		if err != nil {
			applog.Debug("ntp query failed", "component", "timesync", "server", host, "err", err)
			continue
		}
		samples = append(samples, sample{host: host, offset: offset, rtt: rtt})
	}

	measuredAt := time.Now()

	if len(samples) == 0 {
		s.set(ClockRef{Source: SourceNone, MeasuredAt: measuredAt})
		applog.Warn("ntp unreachable; skew detection only",
			"component", "timesync", "servers", len(s.cfg.Servers))
		return
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i].offset < samples[j].offset })
	pick := samples[len(samples)/2]

	ref := ClockRef{
		Offset:     pick.offset,
		RTT:        pick.rtt,
		Source:     "ntp:" + pick.host,
		MeasuredAt: measuredAt,
	}
	s.set(ref)

	if absDuration(ref.Offset) > SkewWarnThreshold {
		applog.Warn("ntp offset exceeds threshold", "component", "timesync",
			"server", pick.host, "offset_ms", ref.Offset.Milliseconds(),
			"rtt_ms", ref.RTT.Milliseconds(), "samples", len(samples))
	} else {
		applog.Info("ntp measure", "component", "timesync",
			"server", pick.host, "offset_ms", ref.Offset.Milliseconds(),
			"rtt_ms", ref.RTT.Milliseconds(), "samples", len(samples))
	}
}

func (s *Syncer) set(ref ClockRef) {
	s.mu.Lock()
	s.ref = ref
	s.mu.Unlock()
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// --- process-wide singleton -------------------------------------------------

var std = New(Config{})

// Start configures and starts the process-wide Syncer. Call once at startup.
func Start(ctx context.Context, cfg Config) {
	next := New(cfg)
	stdMu.Lock()
	prev := std
	std = next
	stdMu.Unlock()

	prev.Stop()
	next.Start(ctx)
}

// Now returns the process-wide corrected time and its ClockRef.
func Now() (time.Time, ClockRef) { return current().Now() }

// Ref returns the process-wide ClockRef.
func Ref() ClockRef { return current().Ref() }

// Stop stops the process-wide Syncer. Call once on shutdown.
func Stop() { current().Stop() }

var stdMu sync.Mutex

func current() *Syncer {
	stdMu.Lock()
	defer stdMu.Unlock()
	return std
}
