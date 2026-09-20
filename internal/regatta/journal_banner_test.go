package regatta

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/persona/journal"
)

func TestJournalStatusBanner_CreatedForStartTimer(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if r.journalBanner == nil {
		t.Fatal("expected a journal status banner for a start timer session")
	}
	if !r.journalBanner.root.Hidden {
		t.Error("expected the journal banner to start hidden")
	}
	if r.stopJournalStatus == nil {
		t.Error("expected a subscription to be recorded for teardown")
	}
}

func TestJournalStatusBanner_NoneForDirector(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	rd := timerSession(t, "rd", root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(rd, sch)

	if r.journalBanner != nil {
		t.Error("expected no journal banner for the Director, which owns no timing file")
	}
}

// refreshJournalBannerFixture builds a Regatta with a live journalBanner, the
// same way startSession would, without going through a real journal.Manager -
// this tests refreshJournalBanner's rendering logic directly against a
// constructed journal.Status, the same way origin_test.go's
// TestInspectOriginCandidate_* tests inspectOriginCandidate directly rather
// than through the fyne.Do-wrapped caller that normally reaches it.
func refreshJournalBannerFixture(t *testing.T) *Regatta {
	t.Helper()
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)
	if r.journalBanner == nil {
		t.Fatal("expected a journal status banner to be built for setup")
	}
	return r
}

func TestRefreshJournalBanner_ShowsWhileRetrying(t *testing.T) {
	r := refreshJournalBannerFixture(t)

	r.refreshJournalBanner(journal.Status{State: journal.StateRetrying, Since: time.Now().Add(-45 * time.Second)})

	if r.journalBanner.root.Hidden {
		t.Fatal("expected the banner to show while retrying")
	}
	txt := r.journalBanner.label.Text
	if !strings.Contains(txt, "shared regatta folder") {
		t.Errorf("banner text = %q, expected it to mention the shared regatta folder", txt)
	}
	if !strings.Contains(txt, "45s") {
		t.Errorf("banner text = %q, expected it to show the elapsed retry time", txt)
	}
}

func TestRefreshJournalBanner_HidesForQueued(t *testing.T) {
	r := refreshJournalBannerFixture(t)

	// Show it first, then confirm Queued - the normal, near-instant in-flight
	// state before either Synced or Retrying - hides it again rather than
	// flashing its own message.
	r.refreshJournalBanner(journal.Status{State: journal.StateRetrying, Since: time.Now()})
	if r.journalBanner.root.Hidden {
		t.Fatal("expected the banner to be showing before the Queued transition")
	}

	r.refreshJournalBanner(journal.Status{State: journal.StateQueued, Since: time.Now()})
	if !r.journalBanner.root.Hidden {
		t.Error("expected Queued to hide the banner, not show a message for it")
	}
}

func TestRefreshJournalBanner_HidesWhenSynced(t *testing.T) {
	r := refreshJournalBannerFixture(t)

	r.refreshJournalBanner(journal.Status{State: journal.StateRetrying, Since: time.Now()})
	if r.journalBanner.root.Hidden {
		t.Fatal("expected the banner to be showing before the Synced transition")
	}

	r.refreshJournalBanner(journal.Status{State: journal.StateSynced, Since: time.Now()})
	if !r.journalBanner.root.Hidden {
		t.Error("expected Synced to hide the banner")
	}
}

