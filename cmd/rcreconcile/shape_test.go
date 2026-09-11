package main

import (
	"encoding/json"
	"testing"
)

func TestCollectShape(t *testing.T) {
	var v any
	if err := json.Unmarshal([]byte(`{
		"regatta": {"name": "Test Regatta", "id": 42},
		"entries": [
			{"id": 1, "organization": {"name": "Example"}},
			{"id": 2, "organization": {"name": "Other"}}
		],
		"empty": [],
		"emptyObj": {},
		"flag": true,
		"nothing": null
	}`), &v); err != nil {
		t.Fatal(err)
	}

	shape := map[string]string{}
	collectShape(v, "", shape)

	want := map[string]string{
		"regatta.name":                "string",
		"regatta.id":                  "number",
		"entries[].id":                "number",
		"entries[].organization.name": "string",
		"empty[]":                     "array{}",
		"emptyObj":                    "object{}",
		"flag":                        "bool",
		"nothing":                     "null",
	}
	for path, wantType := range want {
		if got, ok := shape[path]; !ok || got != wantType {
			t.Errorf("shape[%q] = %q, ok=%v, want %q", path, got, ok, wantType)
		}
	}
	// Arrays are sampled at element [0] only - a second entries[] field must
	// not appear, since that would mean every element got walked.
	if len(shape) != len(want) {
		t.Errorf("shape has %d entries, want exactly %d: %v", len(shape), len(want), shape)
	}
}

func TestCollectShapeTopLevelScalar(t *testing.T) {
	shape := map[string]string{}
	collectShape("hello", "", shape)
	if shape["."] != "string" {
		t.Errorf("shape[.] = %q, want string", shape["."])
	}
}
