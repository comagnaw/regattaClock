package journal

import (
	"encoding/json"
	"os"
	"time"

	"github.com/comagnaw/regattaClock/internal/applog"
)

// peekedEnvelope mirrors the subset of store.Envelope's fields (RegattaKey,
// Sequence) that a JSON document written by SaveStart/SaveFinish always has at
// its top level, because Envelope is embedded anonymously in StartLog and
// FinishLog. journal never imports store - store imports journal - so this is
// a narrow, independent read of the same wire format rather than a shared
// type.
type peekedEnvelope struct {
	RegattaKey string
	Sequence   int
}

func peekFile(path string) (peekedEnvelope, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return peekedEnvelope{}, false
	}
	return peekBytes(b)
}

func peekBytes(b []byte) (peekedEnvelope, bool) {
	var p peekedEnvelope
	if err := json.Unmarshal(b, &p); err != nil {
		return peekedEnvelope{}, false
	}
	return p, true
}

// recoverPending runs once at Manager construction. If the local journal file
// holds a write that never reached the shared path before an unclean
// shutdown, it is adopted as the pending value and queued for an immediate
// flush; Recovered reports it once so a caller can also refresh its own
// in-memory copy.
func (m *Manager) recoverPending() {
	local, err := os.ReadFile(m.localPath)
	if err != nil {
		return // nothing left over from a crash
	}

	localEnv, ok := peekBytes(local)
	if !ok {
		applog.Warn("journal: local entry unreadable, leaving it in place",
			"component", "journal", "file", m.localPath)
		return
	}
	if localEnv.RegattaKey != m.regattaKey {
		applog.Warn("journal: local entry belongs to a different regatta, leaving it in place",
			"component", "journal", "file", m.localPath, "had", localEnv.RegattaKey, "want", m.regattaKey)
		return
	}

	if sharedEnv, ok := peekFile(m.sharedPath); ok && sharedEnv.RegattaKey == m.regattaKey && sharedEnv.Sequence >= localEnv.Sequence {
		// The shared write actually succeeded before the crash; the local
		// copy is a harmless leftover.
		if err := os.Remove(m.localPath); err != nil {
			applog.Warn("journal: could not remove a stale local entry",
				"component", "journal", "file", m.localPath, "err", err)
		}
		return
	}

	applog.Warn("journal: restored a write that never reached the shared path",
		"component", "journal", "file", m.sharedPath, "sequence", localEnv.Sequence)

	m.mu.Lock()
	m.pending = local
	m.pendingVersion++
	m.recovered = local
	m.status = Status{State: StateQueued, Since: time.Now()}
	m.mu.Unlock()
	m.poke()
}

// Recovered returns a locally-staged write left over from an unclean shutdown
// that Manager adopted as pending at construction (see recoverPending), so a
// caller - persona_startup's hydrate code - can refresh its own in-memory copy
// to match. It reports the finding once: ok is false on every call after the
// first, and always false when there was nothing to recover.
func (m *Manager) Recovered() (b []byte, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recovered == nil {
		return nil, false
	}
	b, m.recovered = m.recovered, nil
	return b, true
}
