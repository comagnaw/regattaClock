package regatta

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
)

var publishNow = time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)

func approvedFinish(key string) *store.FinishLog {
	fin := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "06:00.0", Approved: true, Rows: []store.LapRow{
			{Lane: 1, Place: "1", Split: "03:00.0", Time: "06:00.0"},
			{Lane: 2, Place: "2", Split: "03:02.0", Time: "06:04.0"},
		}},
		2: {RaceNumber: 2, WinningTime: "05:00.0"}, // saved, not approved
	}}
	fin.RegattaKey = key
	return fin
}

// startedPublisher starts a session as id on twoRaceSchedule with race 1
// approved and race 2 saved-but-unapproved in the primary finish.json.
func startedPublisher(t *testing.T, id string) *Regatta {
	t.Helper()
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)
	if err := store.SaveFinish(timerSession(t, "pft", root), approvedFinish(store.RegattaKey(sch.Name, sch.Date))); err != nil {
		t.Fatal(err)
	}
	app := test.NewTempApp(t)
	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(timerSession(t, id, root), sch)
	return r
}

func TestPublishButton_PrimaryFinishTimerOnly(t *testing.T) {
	if r := startedPublisher(t, "pft"); r.rows[1].publishBtn == nil {
		t.Error("pft row has no Publish button")
	}
	for _, id := range []string{"sft", "pst", "awd"} {
		r := startedPublisher(t, id)
		if r.rows[1].publishBtn != nil {
			t.Errorf("%s row has a Publish button, want only the primary finish timer", id)
		}
	}
}

func TestPublishButton_StateFollowsApprovalAndLedger(t *testing.T) {
	r := startedPublisher(t, "pft")

	if b := r.rows[1].publishBtn; b.Disabled() || b.Text != common.PublishButtonText {
		t.Errorf("approved, unpublished: %q disabled=%v, want enabled Publish", b.Text, b.Disabled())
	}
	if b := r.rows[2].publishBtn; !b.Disabled() {
		t.Error("unapproved race: Publish enabled, want disabled")
	}

	rev := r.raceRevision(1, r.finishLog.Races[1])
	r.applyLedger(spreadsheet.Ledger{1: {Revision: rev}})
	if b := r.rows[1].publishBtn; b.Text != common.PublishedButtonText || b.Importance == widget.HighImportance {
		t.Errorf("published, current: %q importance=%v, want Published", b.Text, b.Importance)
	}

	r.applyLedger(spreadsheet.Ledger{1: {Revision: "an-older-rev"}})
	if b := r.rows[1].publishBtn; b.Text != common.RepublishButtonText || b.Importance != widget.HighImportance {
		t.Errorf("published, stale: %q importance=%v, want highlighted Re-publish", b.Text, b.Importance)
	}

	r.setPublishing(true)
	if !r.rows[1].publishBtn.Disabled() {
		t.Error("Publish enabled while a publish is in flight")
	}
}

func TestDerivePublishState(t *testing.T) {
	tests := []struct {
		name                  string
		canPublish, published bool
		rev, publishedRev     string
		want                  publishState
	}{
		{"unapproved", false, false, "", "", publishState{label: common.PublishButtonText}},
		{"unapproved but published", false, true, "", "r1", publishState{label: common.PublishedButtonText}},
		{"approved new", true, false, "r1", "", publishState{label: common.PublishButtonText, enabled: true}},
		{"approved current", true, true, "r1", "r1", publishState{label: common.PublishedButtonText, enabled: true}},
		{"approved stale", true, true, "r2", "r1", publishState{label: common.RepublishButtonText, enabled: true, stale: true}},
	}
	for _, tt := range tests {
		if got := derivePublishState(tt.canPublish, tt.rev, tt.publishedRev, tt.published); got != tt.want {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func publishViews(fin *store.FinishLog) (skeleton, approved []publish.PublishableRace) {
	sch := twoRaceSchedule()
	return publish.ScheduleView(sch), publish.BuildView(sch, fin)
}

func TestMergePublished(t *testing.T) {
	fin := approvedFinish("k")
	skeleton, approved := publishViews(fin)
	earlier := publishNow.Add(-time.Hour)

	t.Run("first publish fills only that race", func(t *testing.T) {
		races, ledger := mergePublished(1, skeleton, approved, spreadsheet.Published{}, publishNow)
		if len(races) != 2 || len(races[0].Rows) != 2 || races[1].Rows != nil {
			t.Fatalf("races = %+v", races)
		}
		if e := ledger[1]; e.Revision != approved[0].Revision || !e.PublishedAt.Equal(publishNow) || len(ledger) != 1 {
			t.Errorf("ledger = %+v", ledger)
		}
	})

	t.Run("unchanged republish keeps publishedAt", func(t *testing.T) {
		prev := spreadsheet.Published{Ledger: spreadsheet.Ledger{1: {Revision: approved[0].Revision, PublishedAt: earlier}}}
		_, ledger := mergePublished(1, skeleton, approved, prev, publishNow)
		if !ledger[1].PublishedAt.Equal(earlier) {
			t.Errorf("PublishedAt = %v, want the original %v", ledger[1].PublishedAt, earlier)
		}
	})

	t.Run("stale ledger race refreshed alongside", func(t *testing.T) {
		prev := spreadsheet.Published{Ledger: spreadsheet.Ledger{1: {Revision: "old", PublishedAt: earlier}}}
		_, ledger := mergePublished(2, skeleton, approved, prev, publishNow)
		if e := ledger[1]; e.Revision != approved[0].Revision || !e.PublishedAt.Equal(publishNow) {
			t.Errorf("ledger[1] = %+v, want re-stamped to the current revision", e)
		}
		if _, ok := ledger[2]; ok {
			t.Error("unapproved race 2 entered the ledger")
		}
	})

	t.Run("un-approved ledger race carried forward", func(t *testing.T) {
		kept := []publish.Row{{Lane: "1", Place: "1", Time: "05:00.0"}}
		prev := spreadsheet.Published{
			Ledger: spreadsheet.Ledger{2: {Revision: "r2", PublishedAt: earlier}},
			Rows:   map[int][]publish.Row{2: kept},
		}
		races, ledger := mergePublished(1, skeleton, approved, prev, publishNow)
		if len(races[1].Rows) != 1 || races[1].Rows[0] != kept[0] {
			t.Errorf("race 2 rows = %+v, want the previously published rows", races[1].Rows)
		}
		if ledger[2].Revision != "r2" {
			t.Errorf("ledger[2] = %+v, want kept as published", ledger[2])
		}
	})
}

func TestPublishResults_WritesAndRereads(t *testing.T) {
	skeleton, approved := publishViews(approvedFinish("k"))
	path := filepath.Join(t.TempDir(), spreadsheet.FileName("Lock Test"))
	meta := spreadsheet.Meta{Name: "Lock Test", Date: "2026-10-02"}

	ledger, err := publishResults(path, 1, meta, skeleton, approved, publishNow)
	if err != nil {
		t.Fatalf("publishResults() error = %v", err)
	}
	got, err := spreadsheet.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ledger[1].Revision != ledger[1].Revision || len(got.Rows[1]) != 2 {
		t.Errorf("read back %+v, want race 1 published", got)
	}
}

func TestPublishResults_ForeignWorkbookUntouched(t *testing.T) {
	skeleton, approved := publishViews(approvedFinish("k"))
	path := filepath.Join(t.TempDir(), spreadsheet.FileName("Lock Test"))
	if err := os.WriteFile(path, []byte("not ours"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := publishResults(path, 1, spreadsheet.Meta{}, skeleton, approved, publishNow)
	if err == nil {
		t.Fatal("publishResults() over a foreign file succeeded")
	}
	if data, _ := os.ReadFile(path); string(data) != "not ours" {
		t.Error("foreign file was overwritten")
	}
}

func TestPublishErrorMessage(t *testing.T) {
	if got := publishErrorMessage("r.xlsx", spreadsheet.ErrLocked); got == spreadsheet.ErrLocked.Error() {
		t.Errorf("locked message = %q, want the close-and-retry copy", got)
	}
	if got := publishErrorMessage("r.xlsx", errors.New("boom")); got != "boom" {
		t.Errorf("default message = %q", got)
	}
}
