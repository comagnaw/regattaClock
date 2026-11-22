package destinations

import "testing"

func TestKinds_AvailableOnesAreBuilt(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Kinds {
		if k.ID == "" || k.Label == "" {
			t.Errorf("kind %+v has no ID or label", k)
		}
		if seen[k.ID] {
			t.Errorf("duplicate kind %q", k.ID)
		}
		seen[k.ID] = true
		if k.Available != (k.New != nil) {
			t.Errorf("%s: Available=%v but New set=%v", k.ID, k.Available, k.New != nil)
		}
	}
}

func TestByID(t *testing.T) {
	if k, ok := ByID("webjson"); !ok || !k.NeedsFolder {
		t.Errorf("ByID(webjson) = %+v, %v", k, ok)
	}
	if _, ok := ByID("carrier-pigeon"); ok {
		t.Error("ByID found an unknown kind")
	}
}
