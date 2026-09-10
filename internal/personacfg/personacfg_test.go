package personacfg

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "personas.json")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantErr    bool
		wantNotEx  bool // errors.Is(err, fs.ErrNotExist)
		wantHosts  int
		wantChalls int
	}{
		{name: "both sections", body: `{"hosts":{"pc-1":"pst"},"challenges":{"pft":"code"}}`, wantHosts: 1, wantChalls: 1},
		{name: "hosts only", body: `{"hosts":{"pc-1":"pst","pc-2":"rd"}}`, wantHosts: 2},
		{name: "challenges only", body: `{"challenges":{"pst":"a","rd":"b"}}`, wantChalls: 2},
		{name: "empty object", body: `{}`},
		{name: "empty file", body: ``, wantErr: true},
		{name: "bad json", body: `{ not json`, wantErr: true},
		{name: "unknown id in hosts", body: `{"hosts":{"pc-1":"xyz"}}`, wantErr: true},
		{name: "unknown id in challenges", body: `{"challenges":{"xyz":"c"}}`, wantErr: true},
		{name: "blank challenge value", body: `{"challenges":{"pst":"  "}}`, wantErr: true},
		{name: "blank host persona id", body: `{"hosts":{"pc-1":""}}`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got cfg %+v", cfg)
				}
				if cfg != nil {
					t.Errorf("expected nil config on error, got %+v", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cfg.Hosts) != tc.wantHosts {
				t.Errorf("hosts = %d, want %d", len(cfg.Hosts), tc.wantHosts)
			}
			if len(cfg.Challenges) != tc.wantChalls {
				t.Errorf("challenges = %d, want %d", len(cfg.Challenges), tc.wantChalls)
			}
		})
	}
}

func TestLoad_MissingFileIsNotExist(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected the error to wrap fs.ErrNotExist, got %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config, got %+v", cfg)
	}
}

func TestLoad_BadFileIsNotNotExist(t *testing.T) {
	_, err := Load(writeConfig(t, `{ not json`))
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a malformed file must not look like a missing one: %v", err)
	}
}

func TestConfigAssignmentFor(t *testing.T) {
	cfg := &Config{Hosts: map[string]string{
		"Timer-01":  "pst",
		" rd-box  ": "rd",
	}}

	tests := []struct {
		host   string
		wantID string
		wantOK bool
	}{
		{"timer-01", "pst", true},
		{"TIMER-01", "pst", true},
		{"Timer-01.regatta.local", "pst", true},
		{"rd-box", "rd", true},
		{"other", "", false},
		{"", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			def, ok := cfg.AssignmentFor(tc.host)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && def.ID != tc.wantID {
				t.Errorf("id = %q, want %q", def.ID, tc.wantID)
			}
		})
	}

	if _, ok := (&Config{}).AssignmentFor("anything"); ok {
		t.Error("an empty config must not assign a persona")
	}
	if _, ok := (*Config)(nil).AssignmentFor("anything"); ok {
		t.Error("a nil config must not assign a persona")
	}
}

func TestConfigMatchesChallenge(t *testing.T) {
	pst, _ := persona.ByID("pst")
	sst, _ := persona.ByID("sst")
	cfg := &Config{Challenges: map[string]string{"pst": "LetMeIn"}}

	tests := []struct {
		name  string
		def   persona.Definition
		input string
		want  bool
	}{
		{"override accepted", pst, "letmein", true},
		{"override case/space insensitive", pst, "  LETMEIN ", true},
		{"built-in rejected once overridden", pst, "rc-pst", false},
		{"empty input rejected", pst, "", false},
		{"non-overridden persona keeps built-in", sst, "rc-sst", true},
		{"non-overridden persona rejects wrong", sst, "nope", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.MatchesChallenge(tc.def, tc.input); got != tc.want {
				t.Errorf("MatchesChallenge(%q, %q) = %v, want %v", tc.def.ID, tc.input, got, tc.want)
			}
		})
	}

	if !(&Config{}).MatchesChallenge(pst, "rc-pst") {
		t.Error("an empty config must delegate to the built-in code")
	}
}

func TestLoad_RegattaCentral(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "no section", body: `{"hosts":{"pc-1":"pst"}}`},
		{name: "id only", body: `{"regattacentral":{"regattaID":"12345"}}`},
		{name: "id and valid baseURL", body: `{"regattacentral":{"regattaID":"12345","baseURL":"https://api.regattacentral.com/v4.0/"}}`},
		{name: "missing id", body: `{"regattacentral":{"baseURL":"https://api.regattacentral.com/v4.0/"}}`, wantErr: true},
		{name: "blank id", body: `{"regattacentral":{"regattaID":"  "}}`, wantErr: true},
		{name: "relative baseURL", body: `{"regattacentral":{"regattaID":"1","baseURL":"/v4.0/"}}`, wantErr: true},
		{name: "non-http baseURL", body: `{"regattacentral":{"regattaID":"1","baseURL":"ftp://example.com/"}}`, wantErr: true},
		{name: "baseURL no host", body: `{"regattacentral":{"regattaID":"1","baseURL":"https:///v4.0/"}}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got cfg %+v", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfigRegattaCentralConfig(t *testing.T) {
	if _, ok := (*Config)(nil).RegattaCentralConfig(); ok {
		t.Error("nil config must report no RegattaCentral config")
	}
	if _, ok := (&Config{}).RegattaCentralConfig(); ok {
		t.Error("empty config must report no RegattaCentral config")
	}

	cfg, err := Load(writeConfig(t, `{"regattacentral":{"regattaID":"  99 ","baseURL":"  https://rc.example/v4/  "}}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rc, ok := cfg.RegattaCentralConfig()
	if !ok {
		t.Fatal("expected RegattaCentral config to be present")
	}
	if rc.RegattaID != "99" || rc.BaseURL != "https://rc.example/v4/" {
		t.Errorf("values not trimmed: %+v", rc)
	}
}
