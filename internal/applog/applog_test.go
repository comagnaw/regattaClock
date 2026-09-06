package applog

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// resetForTest returns the package singleton to its zero state and arranges for
// the same cleanup after the test, since every test shares the global logger.
func resetForTest(t *testing.T) {
	t.Helper()
	clear := func() {
		mu.Lock()
		defer mu.Unlock()
		if writer != nil {
			writer.close()
			writer = nil
		}
		lvl.Set(slog.LevelInfo)
		loggingOn = false
		outPath = ""
		ident = identity{}
		active.Store(discard)
	}
	clear()
	t.Cleanup(clear)
}

func droppedSoFar() int64 {
	mu.Lock()
	defer mu.Unlock()
	if writer == nil {
		return 0
	}
	return writer.dropped.Load()
}

func readJSONLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line is not valid JSON (%q): %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func messages(lines []map[string]any) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i], _ = l["msg"].(string)
	}
	return out
}

func TestDisabledWritesNothing(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(false, false)
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Info("nope", "k", "v")
	Debug("also nope")
	Close()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no log file to be created, stat err = %v", err)
	}
}

func TestInfoLineShape(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(true, false)
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Info("hello", "component", "test", "n", 12)
	Close()

	lines := readJSONLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %v", len(lines), lines)
	}
	l := lines[0]
	if l["level"] != "INFO" || l["msg"] != "hello" || l["component"] != "test" {
		t.Fatalf("unexpected line: %v", l)
	}
	if n, _ := l["n"].(float64); n != 12 {
		t.Fatalf("want n=12, got %v", l["n"])
	}
	if _, ok := l["time"]; !ok {
		t.Fatalf("line has no time field: %v", l)
	}
}

func TestDebugGatedByDebugPreference(t *testing.T) {
	resetForTest(t)

	offPath := filepath.Join(t.TempDir(), "off.log")
	Init(true, false)
	if err := SetOutput(offPath); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Debug("d1")
	Info("i1")
	Close()
	if got := messages(readJSONLines(t, offPath)); len(got) != 1 || got[0] != "i1" {
		t.Fatalf("debug off: want [i1], got %v", got)
	}

	resetForTest(t)
	onPath := filepath.Join(t.TempDir(), "on.log")
	Init(true, true)
	if err := SetOutput(onPath); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Debug("d2")
	Info("i2")
	Close()
	lines := readJSONLines(t, onPath)
	if got := messages(lines); len(got) != 2 || got[0] != "d2" || got[1] != "i2" {
		t.Fatalf("debug on: want [d2 i2], got %v", got)
	}
	if lines[0]["level"] != "DEBUG" {
		t.Fatalf("want first line level DEBUG, got %v", lines[0]["level"])
	}
}

func TestIdentityAttrs(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(true, false)
	SetIdentity("pst", "primary", "start", "HOST1")
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Info("x")
	Close()

	l := readJSONLines(t, path)[0]
	for k, want := range map[string]string{
		"persona_id": "pst", "team": "primary", "role": "start", "machine": "HOST1",
	} {
		if l[k] != want {
			t.Fatalf("want %s=%q, got %q (line %v)", k, want, l[k], l)
		}
	}
}

func TestIdentityOmitsUnsetFields(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(true, false)
	SetIdentity("", "", "", "ONLYHOST")
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Info("x")
	Close()

	l := readJSONLines(t, path)[0]
	if l["machine"] != "ONLYHOST" {
		t.Fatalf("want machine=ONLYHOST, got %v", l["machine"])
	}
	for _, k := range []string{"persona_id", "team", "role"} {
		if _, present := l[k]; present {
			t.Fatalf("expected %s to be omitted, line = %v", k, l)
		}
	}
}

func TestBufferedLinesReplayedOnSetOutput(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(true, false)
	Info("early-1")
	Info("early-2")
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Info("late-1")
	Close()

	got := messages(readJSONLines(t, path))
	want := []string{"early-1", "early-2", "late-1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestSetLevelTurnsLoggingOnMidSession(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(false, false)
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Info("suppressed")
	SetLevel(true, false)
	Info("kept")
	Close()

	if got := messages(readJSONLines(t, path)); len(got) != 1 || got[0] != "kept" {
		t.Fatalf("want [kept], got %v", got)
	}
}

func TestSetLevelTurnsLoggingOffThenBackOn(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(true, false)
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}
	Info("a")
	SetLevel(false, false)
	Info("b") // discarded while off
	SetLevel(true, false)
	Info("c")
	Close()

	got := messages(readJSONLines(t, path))
	want := []string{"a", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestConcurrentLoggingNoLossNoTear(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(true, false)
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}

	const goroutines, perGoroutine = 8, 500
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				Info("evt", "g", g, "i", i)
			}
		}(g)
	}
	wg.Wait()

	dropped := droppedSoFar()
	Close()

	written := int64(len(readJSONLines(t, path)))
	if written+dropped != goroutines*perGoroutine {
		t.Fatalf("written(%d) + dropped(%d) != %d", written, dropped, goroutines*perGoroutine)
	}
	if written == 0 {
		t.Fatal("expected some lines to be written")
	}
}

func TestHighVolumeDropsRatherThanBlocks(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "x.log")

	Init(true, false)
	if err := SetOutput(path); err != nil {
		t.Fatalf("SetOutput: %v", err)
	}

	const n = 20000
	for i := 0; i < n; i++ {
		Info("v", "i", i)
	}
	dropped := droppedSoFar()
	Close()

	written := int64(len(readJSONLines(t, path)))
	if written+dropped != n {
		t.Fatalf("written(%d) + dropped(%d) != %d", written, dropped, n)
	}
	if written == 0 {
		t.Fatal("expected a substantial number of lines to be written")
	}
}

func TestCloseIsSafeWithoutInit(t *testing.T) {
	resetForTest(t)
	Close()             // must not panic
	Info("after close") // discarded, must not panic
}
