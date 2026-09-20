package regatta

import (
	"os"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona/journal"
)

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
	journal.Configure(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
