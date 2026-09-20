package regatta

import (
	"os"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona/journal"
)

// journalTestRoot is the scratch directory TestMain points the journal at,
// exposed so a crash-recovery test elsewhere in this package (see
// journal_recovery_test.go) can seed a local journal entry directly on disk -
// simulating a leftover from a run that crashed before this process started.
// journal.For itself cannot do this: its one-time crash-recovery check only
// ever runs at a path's first construction in a process, so using it to seed
// would consume that one chance instead of leaving it for hydrateOwnStart's
// own later call.
var journalTestRoot string

// TestMain points the write-ahead journal at a scratch directory for the
// whole test binary, so store.SaveStart/SaveFinish (exercised throughout this
// package, directly and via the ST/FT flows) never touch the real OS cache
// directory during a test run. journal.For namespaces a Manager's local file
// by a hash of the session's Root, so tests sharing this one configured root
// never collide even when they leave RegattaKey at its zero value.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "regattaClock-journal-test-*")
	if err != nil {
		panic(err)
	}
	journalTestRoot = dir
	journal.Configure(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
