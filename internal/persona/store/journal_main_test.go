package store

import (
	"os"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona/journal"
)

// TestMain points the write-ahead journal at a scratch directory for the
// whole test binary, so SaveStart/SaveFinish never touch the real OS cache
// directory during a test run. journal.For namespaces a Manager's local file
// by a hash of the session's Root (see journal/registry.go's rootNamespace),
// so tests sharing this one configured root - even ones that never bother
// setting a real RegattaKey - never collide: each test's t.TempDir() Root
// hashes to a distinct local file.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "regattaClock-journal-test-*")
	if err != nil {
		panic(err)
	}
	journal.Configure(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
