package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
)

var (
	rootMu        sync.Mutex
	configuredDir string

	registryMu sync.Mutex
	registry   = map[string]*Manager{}
)

// Configure sets the local directory journal files are staged under. Call it
// once at startup, before the first persona.Session is created. If never
// called (or called with ""), the root resolves lazily to
// os.UserCacheDir()/regattaClock/journal the first time it's needed - tests
// call Configure(t.TempDir()) instead, so a test run never touches the real
// OS cache directory.
func Configure(dir string) {
	rootMu.Lock()
	configuredDir = dir
	rootMu.Unlock()
}

func rootDir() (string, error) {
	rootMu.Lock()
	dir := configuredDir
	rootMu.Unlock()
	if dir != "" {
		return dir, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("journal: could not resolve a local cache directory: %w", err)
	}
	return filepath.Join(cache, "regattaClock", "journal"), nil
}

// For returns the process-wide Manager for sess's WritePath, creating it (and
// running crash recovery against regattaKey) on first call for that path.
// Memoized so every caller in the process - store.saveLog and the startup
// hydrate code - shares one instance and one background flusher goroutine per
// file.
func For(sess persona.Session, regattaKey string) (*Manager, error) {
	sharedPath := sess.WritePath()
	if sharedPath == "" {
		return nil, fmt.Errorf("journal: %s/%s has no write path", sess.Role, sess.Team)
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if m, ok := registry[sharedPath]; ok {
		return m, nil
	}

	dir, err := rootDir()
	if err != nil {
		return nil, err
	}
	localPath := filepath.Join(dir, rootNamespace(sess.Root), string(sess.Team), string(sess.Role)+".pending.json")

	m := newManager(sharedPath, localPath, regattaKey)
	registry[sharedPath] = m
	return m, nil
}

// rootNamespace keys a Manager's local staging directory to the regattaData
// folder it mirrors (sess.Root), not to the regattaKey the session happens to
// be working with right now. Two Managers for the same folder always land on
// the same local file, which is what lets crash recovery find a leftover
// write after a restart; two Managers for different folders - a different
// regatta's folder, or, in tests, a fresh t.TempDir() per test - never
// collide, even if their regattaKeys happen to coincide (empty in most unit
// tests). The regattaKey mismatch guard in recoverPending is the thing that
// actually protects against "this folder was reused for a different regatta
// since the last write" - this hash only decides which local file two
// Managers share, not whether it's safe to adopt what one finds there.
func rootNamespace(root string) string {
	return filesystem.HashBytes([]byte(root))[:16]
}

func newManager(sharedPath, localPath, regattaKey string) *Manager {
	m := &Manager{
		sharedPath:  sharedPath,
		localPath:   localPath,
		regattaKey:  regattaKey,
		localWrite:  func(b []byte) error { return writeAtomic(localPath, b) },
		sharedWrite: func(b []byte) error { return writeAtomic(sharedPath, b) },
		subs:        make(map[int]func(Status)),
		wake:        make(chan struct{}, 1),
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
		completedCh: make(chan struct{}),
	}
	m.status = Status{State: StateSynced, Since: time.Now()}
	m.recoverPending()
	go m.run()
	return m
}

// writeAtomic creates path's parent directory and writes b through the atomic
// writer, mirroring internal/persona/store's saveJSONAtomic. Both localWrite
// and sharedWrite use it so the two destinations a Manager writes to get
// identical treatment.
func writeAtomic(path string, b []byte) error {
	if err := filesystem.CreateDirs(filepath.Dir(path)); err != nil {
		return err
	}
	return filesystem.SaveBytesFileAtomic(b, path)
}
