package regatta

import (
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish/destinations"
)

func TestPublishDestinationsMenu_PrimaryFinishTimerOnly(t *testing.T) {
	has := func(r *Regatta) bool {
		for _, l := range menuLabels(r.makeMenu()) {
			if l == common.PublishDestinationsTitle {
				return true
			}
		}
		return false
	}
	if !has(startedPublisher(t, "pft")) {
		t.Error("pft menu has no Publish Destinations item")
	}
	for _, id := range []string{"sft", "pst", "awd"} {
		if has(startedPublisher(t, id)) {
			t.Errorf("%s menu has Publish Destinations, want only the primary finish timer", id)
		}
	}
}

func TestPublishDraft_Validate(t *testing.T) {
	ss, wj := store.DestinationSpreadsheet, store.DestinationWebJSON
	tests := []struct {
		name    string
		draft   publishDraft
		want    []string
		problem bool
	}{
		{"nothing ticked", publishDraft{enabled: map[string]bool{}, dirs: map[string]string{}}, nil, true},
		{"folder kind without folder", publishDraft{enabled: map[string]bool{wj: true}, dirs: map[string]string{}}, nil, true},
		{"registry order, not tick order",
			publishDraft{enabled: map[string]bool{wj: true, ss: true}, dirs: map[string]string{wj: "/w", ss: "/s"}},
			[]string{ss, wj}, false},
	}
	for _, tt := range tests {
		got, problem := tt.draft.validate()
		if !reflect.DeepEqual(got, tt.want) || (problem != "") != tt.problem {
			t.Errorf("%s: validate() = %v, %q", tt.name, got, problem)
		}
	}
}

func TestSetPublishSettings_RecordsKindsFoldersAndPrefs(t *testing.T) {
	sch := twoRaceSchedule()
	legacy := t.TempDir()
	r := startedPublisherWith(t, sch, seedRegatta(t, sch), legacy, "")
	web := t.TempDir()
	kinds := []string{store.DestinationSpreadsheet, store.DestinationWebJSON}

	if !r.setPublishSettings(kinds, map[string]string{store.DestinationSpreadsheet: legacy, store.DestinationWebJSON: web}) {
		t.Fatal("setPublishSettings() = false")
	}
	quiescePublisher(t, r)

	saved := r.hydrateOwnFinish(r.session, r.regattaKey) // re-read from disk
	if got := saved.PublishDestinations(); !reflect.DeepEqual(got, kinds) {
		t.Errorf("saved destinations = %v, want %v", got, kinds)
	}
	if saved.PublishDir(store.DestinationWebJSON) != web || saved.PublishDir(store.DestinationSpreadsheet) != legacy {
		t.Errorf("saved dirs = %+v", saved.Publish)
	}
	if got := r.App.Preferences().String(resultsDirPref(store.DestinationWebJSON)); got != web {
		t.Errorf("web last-used folder = %q, want %q", got, web)
	}
	summary, _ := r.publishSummary.Get()
	if !strings.Contains(summary, common.DestinationWebJSONLabel) || !strings.Contains(summary, web) {
		t.Errorf("Configuration summary = %q, want the web destination and its folder", summary)
	}
}

func TestSetPublishSettings_RefusedWhileWritesBlocked(t *testing.T) {
	sch := twoRaceSchedule()
	r := startedPublisherWith(t, sch, seedRegatta(t, sch), "", "")
	r.writesBlocked = true

	if r.setPublishSettings([]string{store.DestinationWebJSON}, map[string]string{store.DestinationWebJSON: t.TempDir()}) {
		t.Fatal("setPublishSettings() recorded while finish.json writes are blocked")
	}
	if got := r.finishLog.PublishDestinations(); !reflect.DeepEqual(got, []string{store.DestinationSpreadsheet}) {
		t.Errorf("destinations = %v, want unchanged", got)
	}
}

func TestSetPublishSettings_NotAPublisher(t *testing.T) {
	r := startedPublisher(t, "sft")
	if r.setPublishSettings([]string{store.DestinationWebJSON}, nil) {
		t.Error("setPublishSettings() recorded for the secondary finish timer")
	}
	summary, _ := r.publishSummary.Get()
	if summary != common.PublishSummaryNotPFT {
		t.Errorf("summary = %q, want the PFT-only note", summary)
	}
}

func TestDestinationRow_UnavailableKindIsDisabled(t *testing.T) {
	r := startedPublisher(t, "pft")
	rc, _ := destinations.ByID(store.DestinationRegattaCentral)
	check, ok := r.destinationRow(rc, &publishDraft{enabled: map[string]bool{}, dirs: map[string]string{}}).(*widget.Check)
	if !ok {
		t.Fatal("a folderless kind's row is not a bare checkbox")
	}
	if !check.Disabled() || !strings.Contains(check.Text, "coming soon") {
		t.Errorf("RegattaCentral row: %q disabled=%v, want disabled, coming soon", check.Text, check.Disabled())
	}
}
