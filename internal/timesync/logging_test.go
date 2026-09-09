package timesync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/applog"
)

// captureLog runs fn with applog writing JSON lines to a temp file and returns
// the parsed lines.
func captureLog(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "applog.log")
	applog.Init(true, false)
	if err := applog.SetOutput(path); err != nil {
		t.Fatalf("applog.SetOutput: %v", err)
	}
	t.Cleanup(applog.Close)

	fn()
	applog.Close()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read applog file: %v", err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("applog line not JSON (%q): %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func findLine(lines []map[string]any, msg string) map[string]any {
	for _, l := range lines {
		if l["msg"] == msg {
			return l
		}
	}
	return nil
}

func TestMeasurementLogsInfoBelowThreshold(t *testing.T) {
	lines := captureLog(t, func() {
		s := New(Config{
			Servers:   []string{"a"},
			QueryFunc: stubQuery(map[string]time.Duration{"a": 200 * time.Millisecond}),
		})
		s.measureOnce(context.Background())
	})

	l := findLine(lines, "ntp measure")
	if l == nil {
		t.Fatalf("no 'ntp measure' line: %v", lines)
	}
	if l["level"] != "INFO" || l["component"] != "timesync" || l["server"] != "a" {
		t.Fatalf("unexpected line: %v", l)
	}
	if findLine(lines, "ntp offset exceeds threshold") != nil {
		t.Fatal("did not expect a threshold warning for a 200ms offset")
	}
}

func TestMeasurementLogsWarnAboveThreshold(t *testing.T) {
	lines := captureLog(t, func() {
		s := New(Config{
			Servers:   []string{"a"},
			QueryFunc: stubQuery(map[string]time.Duration{"a": 3 * time.Second}),
		})
		s.measureOnce(context.Background())
	})

	l := findLine(lines, "ntp offset exceeds threshold")
	if l == nil {
		t.Fatalf("no threshold warning: %v", lines)
	}
	if l["level"] != "WARN" {
		t.Fatalf("level = %v, want WARN", l["level"])
	}
}

func TestUnreachableLogsWarn(t *testing.T) {
	lines := captureLog(t, func() {
		s := New(Config{
			Servers:   []string{"a", "b"},
			QueryFunc: stubQuery(nil, "a", "b"),
		})
		s.measureOnce(context.Background())
	})

	l := findLine(lines, "ntp unreachable; skew detection only")
	if l == nil {
		t.Fatalf("no unreachable warning: %v", lines)
	}
	if l["level"] != "WARN" {
		t.Fatalf("level = %v, want WARN", l["level"])
	}
}
