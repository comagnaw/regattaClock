package watcher

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

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func startWatch(t *testing.T, mode Mode, interval time.Duration, paths ...string) (<-chan Event, *Watcher) {
	t.Helper()
	w := New(mode, interval)
	for _, p := range paths {
		w.Add(p)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := w.Start(ctx)
	t.Cleanup(func() {
		cancel()
		w.Stop()
	})
	return ch, w
}

func waitEvent(t *testing.T, ch <-chan Event, within time.Duration) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed unexpectedly")
		}
		return ev
	case <-time.After(within):
		t.Fatal("timed out waiting for a watcher event")
		return Event{}
	}
}

func expectNoEvent(t *testing.T, ch <-chan Event, within time.Duration) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("unexpected event: path=%s data=%q", ev.Path, ev.Data)
		}
	case <-time.After(within):
	}
}

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
		t.Fatalf("read applog: %v", err)
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

func linesWithMsg(lines []map[string]any, msg string) []map[string]any {
	var out []map[string]any
	for _, l := range lines {
		if l["msg"] == msg {
			out = append(out, l)
		}
	}
	return out
}

func TestEmitsCurrentContentOnStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.json")
	write(t, path, `{"v":1}`)

	ch, _ := startWatch(t, ModeCloud, 20*time.Millisecond, path)

	ev := waitEvent(t, ch, 2*time.Second)
	if ev.Path != path || string(ev.Data) != `{"v":1}` {
		t.Fatalf("event = %+v", ev)
	}
}

func TestEmitsOnContentChangeNotOnIdenticalRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.json")
	write(t, path, "v1")

	ch, _ := startWatch(t, ModeCloud, 20*time.Millisecond, path)
	waitEvent(t, ch, 2*time.Second) // baseline

	write(t, path, "v2")
	if ev := waitEvent(t, ch, 2*time.Second); string(ev.Data) != "v2" {
		t.Fatalf("want v2, got %q", ev.Data)
	}

	// Rewrite identical content with a bumped mtime: no event.
	write(t, path, "v2")
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	expectNoEvent(t, ch, 300*time.Millisecond)
}

func TestNoEventForMissingFileThenCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.json")

	ch, _ := startWatch(t, ModeCloud, 20*time.Millisecond, path)
	expectNoEvent(t, ch, 200*time.Millisecond)

	write(t, path, "born")
	if ev := waitEvent(t, ch, 2*time.Second); string(ev.Data) != "born" {
		t.Fatalf("want born, got %q", ev.Data)
	}
}

func TestOlderMtimeStillDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.json")
	write(t, path, "v1")

	ch, _ := startWatch(t, ModeCloud, 20*time.Millisecond, path)
	waitEvent(t, ch, 2*time.Second)

	// Cloud sync can land a new version stamped with an older mtime.
	write(t, path, "v2-from-cloud")
	past := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	if ev := waitEvent(t, ch, 2*time.Second); string(ev.Data) != "v2-from-cloud" {
		t.Fatalf("want v2-from-cloud, got %q", ev.Data)
	}
}

func TestReadFailureNoEventAndWarnsOnce(t *testing.T) {
	dir := t.TempDir()
	// A directory at the watched path: os.Stat succeeds, os.ReadFile fails.
	asDir := filepath.Join(dir, "start.json")
	if err := os.Mkdir(asDir, 0755); err != nil {
		t.Fatal(err)
	}

	var ch <-chan Event
	lines := captureLog(t, func() {
		var w *Watcher
		ch, w = startWatch(t, ModeCloud, 15*time.Millisecond, asDir)
		expectNoEvent(t, ch, 250*time.Millisecond)
		_ = w
	})

	warns := linesWithMsg(lines, "watcher read failed")
	if len(warns) != 1 {
		t.Fatalf("want exactly one 'watcher read failed' warning, got %d: %v", len(warns), lines)
	}
	if warns[0]["level"] != "WARN" {
		t.Fatalf("level = %v, want WARN", warns[0]["level"])
	}
}

func TestConflictCopyDetectedInCloudModeOnce(t *testing.T) {
	dir := t.TempDir()
	watched := filepath.Join(dir, "start.json")
	write(t, watched, "real")
	write(t, filepath.Join(dir, "start-DESKTOP-A1B2C3.json"), "conflict")

	var ch <-chan Event
	lines := captureLog(t, func() {
		ch, _ = startWatch(t, ModeCloud, 15*time.Millisecond, watched)
		waitEvent(t, ch, 2*time.Second)
		time.Sleep(150 * time.Millisecond) // several ticks
	})

	warns := linesWithMsg(lines, "conflict copy detected")
	if len(warns) != 1 {
		t.Fatalf("want exactly one conflict warning, got %d: %v", len(warns), warns)
	}
	if !strings.HasSuffix(warns[0]["path"].(string), "start-DESKTOP-A1B2C3.json") || warns[0]["expected"] != "start.json" {
		t.Fatalf("unexpected conflict line: %v", warns[0])
	}
}

func TestConflictCopyIgnoredInSMBMode(t *testing.T) {
	dir := t.TempDir()
	watched := filepath.Join(dir, "finish.json")
	write(t, watched, "real")
	write(t, filepath.Join(dir, "finish (1).json"), "conflict")

	var ch <-chan Event
	lines := captureLog(t, func() {
		ch, _ = startWatch(t, ModeSMB, 15*time.Millisecond, watched)
		waitEvent(t, ch, 2*time.Second)
		time.Sleep(150 * time.Millisecond)
	})

	if got := linesWithMsg(lines, "conflict copy detected"); len(got) != 0 {
		t.Fatalf("SMB mode should not scan for conflict copies, got %v", got)
	}
}

func TestTmpSiblingProducesNothing(t *testing.T) {
	dir := t.TempDir()
	watched := filepath.Join(dir, "start.json")
	write(t, watched, "real")

	var ch <-chan Event
	lines := captureLog(t, func() {
		ch, _ = startWatch(t, ModeCloud, 15*time.Millisecond, watched)
		waitEvent(t, ch, 2*time.Second)
		write(t, filepath.Join(dir, "start.json.tmp"), "half written")
		expectNoEvent(t, ch, 250*time.Millisecond)
	})

	if got := linesWithMsg(lines, "conflict copy detected"); len(got) != 0 {
		t.Fatalf(".tmp sibling should be ignored, got %v", got)
	}
}

func TestIsConflictName(t *testing.T) {
	cases := []struct {
		name, stem string
		want       bool
	}{
		{"start-DESKTOP-A1B2C3.json", "start", true},
		{"start (1).json", "start", true},
		{"start.json.conflict", "start", true},
		{"finish-LAPTOP.json", "finish", true},
		{"startlist.json", "start", false},
		{"started.json", "start", false},
		{"start", "start", false},
	}
	for _, c := range cases {
		if got := isConflictName(c.name, c.stem); got != c.want {
			t.Errorf("isConflictName(%q, %q) = %v, want %v", c.name, c.stem, got, c.want)
		}
	}
}

func TestStopIdempotentAndSafeWithoutStart(t *testing.T) {
	New(ModeCloud, 0).Stop() // no panic, no hang

	w := New(ModeCloud, 20*time.Millisecond)
	w.Add(filepath.Join(t.TempDir(), "start.json"))
	w.Start(context.Background())
	w.Stop()
	w.Stop()
}

func TestChannelClosesWhenContextCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.json")
	write(t, path, "x")

	w := New(ModeCloud, 20*time.Millisecond)
	w.Add(path)
	ctx, cancel := context.WithCancel(context.Background())
	ch := w.Start(ctx)
	waitEvent(t, ch, 2*time.Second)

	cancel()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // closed, as expected
			}
		case <-deadline:
			t.Fatal("event channel was not closed after context cancel")
		}
	}
}

func TestSMBFallsBackToPollWhenNotifyDirMissing(t *testing.T) {
	// Parent directory does not exist yet, so fsnotify cannot Add it.
	path := filepath.Join(t.TempDir(), "timing", "primary", "start.json")

	ch, _ := startWatch(t, ModeSMB, 20*time.Millisecond, path)
	expectNoEvent(t, ch, 150*time.Millisecond)

	write(t, path, "arrived")
	if ev := waitEvent(t, ch, 2*time.Second); string(ev.Data) != "arrived" {
		t.Fatalf("poll fallback missed the create: %q", ev.Data)
	}
}

func TestSMBNotifyDeliversChangeFasterThanPoll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.json")
	write(t, path, "v1")

	// Poll interval far longer than the assertion window: only fsnotify can
	// make this pass in time.
	ch, _ := startWatch(t, ModeSMB, 10*time.Second, path)
	waitEvent(t, ch, 3*time.Second) // baseline (initial checkAll)

	write(t, path, "v2")
	if ev := waitEvent(t, ch, 3*time.Second); string(ev.Data) != "v2" {
		t.Fatalf("want v2 via notify, got %q", ev.Data)
	}
}
