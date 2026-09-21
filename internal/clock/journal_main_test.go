package clock

import (
	"os"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona/journal"
)

// TestMain points the write-ahead journal at a scratch directory for the
// whole test binary, so store.SaveFinish (exercised via recordFirstFinish,
// recordStop, persistFinish, UpdateStartTime) never touches the real OS cache
// directory during a test run. See internal/regatta's journal_main_test.go
// for the same setup and why it's safe to share one configured root across
// every test in the package.
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
