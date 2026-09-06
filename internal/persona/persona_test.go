package persona

import (
	"path/filepath"
	"testing"
)

func TestRegistryIsWellFormed(t *testing.T) {
	roles := map[Role]bool{RoleDirector: true, RoleStart: true, RoleFinish: true}
	teams := map[Team]bool{TeamExecutive: true, TeamPrimary: true, TeamSecondary: true}

	seenID := map[string]bool{}
	seenChallenge := map[string]bool{}
	seenWritePath := map[string]bool{}

	for _, d := range All() {
		if d.ID == "" || d.Label == "" || d.Challenge == "" {
			t.Errorf("%+v has an empty ID, Label, or Challenge", d)
		}
		if !roles[d.Role] {
			t.Errorf("%s: unknown role %q", d.ID, d.Role)
		}
		if !teams[d.Team] {
			t.Errorf("%s: unknown team %q", d.ID, d.Team)
		}

		if seenID[d.ID] {
			t.Errorf("duplicate ID %q", d.ID)
		}
		seenID[d.ID] = true

		norm := normalizeChallenge(d.Challenge)
		if seenChallenge[norm] {
			t.Errorf("duplicate challenge %q", d.Challenge)
		}
		seenChallenge[norm] = true

		// Every persona must resolve to a distinct file it may write.
		s := Session{Definition: d, Root: "/root"}
		wp := s.WritePath()
		if seenWritePath[wp] {
			t.Errorf("%s: write path %q collides with another persona", d.ID, wp)
		}
		seenWritePath[wp] = true

		switch d.Role {
		case RoleDirector:
			if d.File != "" {
				t.Errorf("director File should be empty, got %q", d.File)
			}
			if d.Team != TeamExecutive {
				t.Errorf("director team = %q, want executive", d.Team)
			}
		case RoleStart:
			if d.File != fileStart {
				t.Errorf("%s: File = %q, want %q", d.ID, d.File, fileStart)
			}
		case RoleFinish:
			if d.File != fileFinish {
				t.Errorf("%s: File = %q, want %q", d.ID, d.File, fileFinish)
			}
		}
	}
}

func TestRegistryContents(t *testing.T) {
	want := []Definition{
		{ID: "pst", Role: RoleStart, Team: TeamPrimary, Label: "Primary Start Timer", Challenge: "rc-pst", File: "start.json"},
		{ID: "sst", Role: RoleStart, Team: TeamSecondary, Label: "Secondary Start Timer", Challenge: "rc-sst", File: "start.json"},
		{ID: "pft", Role: RoleFinish, Team: TeamPrimary, Label: "Primary Finish Timer", Challenge: "rc-pft", File: "finish.json"},
		{ID: "sft", Role: RoleFinish, Team: TeamSecondary, Label: "Secondary Finish Timer", Challenge: "rc-sft", File: "finish.json"},
	}
	if len(Registry) != len(want) {
		t.Fatalf("Registry has %d entries, want %d", len(Registry), len(want))
	}
	for i, w := range want {
		if Registry[i] != w {
			t.Errorf("Registry[%d] = %+v, want %+v", i, Registry[i], w)
		}
	}

	rd := DirectorDefinition
	if rd.ID != "rd" || rd.Role != RoleDirector || rd.Team != TeamExecutive || rd.Challenge != "rc-rd" || rd.File != "" {
		t.Errorf("DirectorDefinition = %+v", rd)
	}
}

func TestMatchesChallenge(t *testing.T) {
	pst, _ := ByID("pst")

	cases := []struct {
		in   string
		want bool
	}{
		{"rc-pst", true},
		{"RC-PST", true},
		{"  rc-pst\t", true},
		{"rc-sst", false},
		{"rcpst", false},
		{"", false},
		{"   ", false},
	}
	for _, c := range cases {
		if got := pst.MatchesChallenge(c.in); got != c.want {
			t.Errorf("pst.MatchesChallenge(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	// A blank Challenge (hypothetical malformed entry) must never match blank input.
	if (Definition{}).MatchesChallenge("") {
		t.Error("empty definition matched empty input")
	}
}

func TestByID(t *testing.T) {
	if d, ok := ByID("pft"); !ok || d.Role != RoleFinish || d.Team != TeamPrimary {
		t.Errorf("ByID(pft) = %+v, %v", d, ok)
	}
	if d, ok := ByID("rd"); !ok || d.Role != RoleDirector {
		t.Errorf("ByID(rd) = %+v, %v", d, ok)
	}
	if _, ok := ByID("nope"); ok {
		t.Error("ByID(nope) reported ok")
	}
}

func TestAllReturnsACopy(t *testing.T) {
	a := All()
	if len(a) != len(Registry)+1 {
		t.Fatalf("All() len = %d, want %d", len(a), len(Registry)+1)
	}
	if a[len(a)-1].ID != "rd" {
		t.Errorf("last entry = %q, want rd", a[len(a)-1].ID)
	}

	a[0] = Definition{ID: "mutated"}
	if Registry[0].ID == "mutated" {
		t.Error("mutating All()'s result changed Registry")
	}
}

func TestSessionPaths(t *testing.T) {
	root := filepath.Join("/srv", "regattaData")

	pst, _ := ByID("pst")
	s := Session{Definition: pst, Root: root}

	if got, want := s.SchedulePath(), filepath.Join(root, "director", "regattaSchedule.json"); got != want {
		t.Errorf("SchedulePath = %q, want %q", got, want)
	}
	if got, want := s.StartPath(), filepath.Join(root, "timing", "primary", "start.json"); got != want {
		t.Errorf("StartPath = %q, want %q", got, want)
	}
	if got, want := s.FinishPath(), filepath.Join(root, "timing", "primary", "finish.json"); got != want {
		t.Errorf("FinishPath = %q, want %q", got, want)
	}
	if got, want := s.WritePath(), s.StartPath(); got != want {
		t.Errorf("start timer WritePath = %q, want StartPath %q", got, want)
	}
}

func TestSessionWritePathByRole(t *testing.T) {
	root := filepath.Join("/srv", "regattaData")

	pft, _ := ByID("pft")
	if got, want := (Session{Definition: pft, Root: root}).WritePath(),
		filepath.Join(root, "timing", "primary", "finish.json"); got != want {
		t.Errorf("pft WritePath = %q, want %q", got, want)
	}

	sst, _ := ByID("sst")
	if got, want := (Session{Definition: sst, Root: root}).WritePath(),
		filepath.Join(root, "timing", "secondary", "start.json"); got != want {
		t.Errorf("sst WritePath = %q, want %q", got, want)
	}

	rd := Session{Definition: DirectorDefinition, Root: root}
	if got, want := rd.WritePath(), rd.SchedulePath(); got != want {
		t.Errorf("director WritePath = %q, want SchedulePath %q", got, want)
	}
}
