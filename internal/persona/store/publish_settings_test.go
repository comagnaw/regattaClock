package store

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPublishSettings_DefaultIsSpreadsheetOnly(t *testing.T) {
	for _, f := range []*FinishLog{nil, {}, {Publish: &PublishSettings{}}} {
		if got := f.PublishDestinations(); !reflect.DeepEqual(got, []string{DestinationSpreadsheet}) {
			t.Errorf("PublishDestinations() = %v, want spreadsheet only", got)
		}
	}
}

func TestPublishSettings_LegacyResultsDirIsTheSpreadsheetFolder(t *testing.T) {
	f := &FinishLog{ResultsDir: "/r"}
	if got := f.PublishDir(DestinationSpreadsheet); got != "/r" {
		t.Errorf("PublishDir(spreadsheet) = %q, want the legacy ResultsDir", got)
	}
	if got := f.PublishDir(DestinationWebJSON); got != "" {
		t.Errorf("PublishDir(webjson) = %q, want none", got)
	}
}

func TestPublishSettings_SetMigratesLegacyFolder(t *testing.T) {
	f := &FinishLog{ResultsDir: "/r"}
	f.SetPublishDir(DestinationWebJSON, "/w")

	if f.ResultsDir != "" {
		t.Errorf("ResultsDir = %q, want cleared once migrated", f.ResultsDir)
	}
	if f.PublishDir(DestinationSpreadsheet) != "/r" || f.PublishDir(DestinationWebJSON) != "/w" {
		t.Errorf("Dirs = %v, want the legacy folder kept and the new one added", f.Publish.Dirs)
	}
	if got := f.PublishDestinations(); !reflect.DeepEqual(got, []string{DestinationSpreadsheet}) {
		t.Errorf("PublishDestinations() = %v, want unchanged by a folder edit", got)
	}

	f.SetPublishDestinations([]string{DestinationSpreadsheet, DestinationWebJSON})
	if got := f.PublishDestinations(); len(got) != 2 {
		t.Errorf("PublishDestinations() = %v", got)
	}
}

func TestPublishSettings_CloneIsDeep(t *testing.T) {
	p := &PublishSettings{Destinations: []string{"a"}, Dirs: map[string]string{"a": "/a"}}
	c := p.Clone()
	c.Destinations[0], c.Dirs["a"] = "b", "/b"
	if p.Destinations[0] != "a" || p.Dirs["a"] != "/a" {
		t.Errorf("original changed through the clone: %+v", p)
	}
	if (*PublishSettings)(nil).Clone() != nil {
		t.Error("nil.Clone() != nil")
	}
}

func TestPublishSettings_JSONOmittedWhenUnset(t *testing.T) {
	data, err := json.Marshal(FinishLog{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Publish") {
		t.Errorf("json = %s, want no Publish key when unset", data)
	}
}
