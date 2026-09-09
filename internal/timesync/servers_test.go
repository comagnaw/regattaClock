package timesync

import (
	"slices"
	"testing"
)

func TestParseServers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{",, ,", nil},
		{"time.example.com", []string{"time.example.com"}},
		{"a, b ,c", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{" 192.168.1.10 , time.windows.com ", []string{"192.168.1.10", "time.windows.com"}},
		{"only.trailing,", []string{"only.trailing"}},
	}
	for _, c := range cases {
		if got := ParseServers(c.in); !slices.Equal(got, c.want) {
			t.Errorf("ParseServers(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseServersFeedsDefaults(t *testing.T) {
	// A blank pref must let Config fall back to the public defaults.
	s := New(Config{Servers: ParseServers("")})
	if !slices.Equal(s.cfg.Servers, DefaultServers) {
		t.Fatalf("blank pref did not fall back to DefaultServers: %v", s.cfg.Servers)
	}
}
