package filesystem

import (
	"strings"
	"testing"
)

func TestSanitizeForFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "ordinary name unchanged", input: "Head of the Charles", expected: "Head of the Charles"},
		{name: "reserved characters replaced", input: `a<b>c:d"e/f\g|h?i*j`, expected: "a_b_c_d_e_f_g_h_i_j"},
		{name: "control characters replaced", input: "line\x00one\x1ftwo", expected: "line_one_two"},
		{name: "trailing dots and spaces trimmed", input: "Spring Sprints. . ", expected: "Spring Sprints"},
		{name: "reserved device name prefixed", input: "CON", expected: "_CON"},
		{name: "reserved device name is case insensitive", input: "con", expected: "_con"},
		{name: "reserved device name with extension prefixed", input: "NUL.txt", expected: "_NUL.txt"},
		{name: "reserved COM port prefixed", input: "COM3", expected: "_COM3"},
		{name: "reserved LPT port with extension prefixed", input: "LPT9.log", expected: "_LPT9.log"},
		{name: "device-name substring is fine", input: "CONcert", expected: "CONcert"},
		{name: "empty input", input: "", expected: "_"},
		{name: "fully stripped input", input: " ... ", expected: "_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeForFilename(tt.input); got != tt.expected {
				t.Errorf("SanitizeForFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeForFilename_ProducesWindowsSafeComponent(t *testing.T) {
	got := SanitizeForFilename(`Spring/Sprints: "Heat 1"?`)

	if got == "" {
		t.Fatal("expected a non-empty component")
	}
	if strings.ContainsAny(got, `<>:"/\|?*`) {
		t.Errorf("result %q still contains a Windows-reserved character", got)
	}
	if strings.TrimRight(got, " .") != got {
		t.Errorf("result %q has a trailing dot or space", got)
	}
}
