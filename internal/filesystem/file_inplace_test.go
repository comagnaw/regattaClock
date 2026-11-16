package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// swapOpenHooks replaces the openWithRetry seams for one test and restores
// them (and the backoff) afterwards.
func swapOpenHooks(t *testing.T, open func(string) (*os.File, error), retryable func(error) bool) {
	t.Helper()
	origOpen, origRetryable, origBackoff := openForInPlaceFunc, retryableRename, renameBaseBackoff
	openForInPlaceFunc = open
	retryableRename = retryable
	renameBaseBackoff = time.Microsecond
	t.Cleanup(func() {
		openForInPlaceFunc = origOpen
		retryableRename = origRetryable
		renameBaseBackoff = origBackoff
	})
}

func TestSaveBytesFileInPlace_CreatesNewFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "results.xlsx")
	want := []byte("first publish")

	if err := SaveBytesFileInPlace(want, filename); err != nil {
		t.Fatalf("SaveBytesFileInPlace returned error: %v", err)
	}
	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

// TestSaveBytesFileInPlace_ShorterContentTruncates guards the Truncate: a
// rewrite smaller than the previous file must not leave the old tail behind.
func TestSaveBytesFileInPlace_ShorterContentTruncates(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "results.xlsx")
	if err := os.WriteFile(filename, []byte("a much longer previous workbook"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SaveBytesFileInPlace([]byte("short"), filename); err != nil {
		t.Fatalf("SaveBytesFileInPlace returned error: %v", err)
	}
	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "short" {
		t.Errorf("contents = %q, want %q", got, "short")
	}
}

// TestSaveBytesFileInPlace_KeepsFileIdentity is the point of the function: the
// rewritten file is the same file (same inode / file ID), not a replacement,
// so a sync client sees an edit. No sibling temp file is created either - a
// "*.tmp" is one OneDrive refuses to sync.
func TestSaveBytesFileInPlace_KeepsFileIdentity(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "results.xlsx")
	if err := os.WriteFile(filename, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}

	if err := SaveBytesFileInPlace([]byte("version two"), filename); err != nil {
		t.Fatalf("SaveBytesFileInPlace returned error: %v", err)
	}
	after, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("file identity changed: the rewrite replaced the file instead of editing it")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only results.xlsx", names)
	}
}

func TestSaveBytesFileInPlace_OpenRetriesThenSucceeds(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "results.xlsx")
	held := errors.New("held open")
	var calls int
	swapOpenHooks(t,
		func(name string) (*os.File, error) {
			calls++
			if calls < 3 {
				return nil, held
			}
			return os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0644)
		},
		func(err error) bool { return errors.Is(err, held) },
	)

	if err := SaveBytesFileInPlace([]byte("published"), filename); err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 open attempts, got %d", calls)
	}
}

func TestSaveBytesFileInPlace_OpenGivesUpAndKeepsOldFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "results.xlsx")
	if err := os.WriteFile(filename, []byte("previous"), 0644); err != nil {
		t.Fatal(err)
	}
	held := errors.New("still held")
	var calls int
	swapOpenHooks(t,
		func(string) (*os.File, error) { calls++; return nil, held },
		func(err error) bool { return errors.Is(err, held) },
	)

	err := SaveBytesFileInPlace([]byte("new"), filename)
	if !errors.Is(err, held) {
		t.Fatalf("expected the held error back, got %v", err)
	}
	if calls != renameMaxAttempts {
		t.Errorf("expected %d open attempts, got %d", renameMaxAttempts, calls)
	}
	if got, _ := os.ReadFile(filename); string(got) != "previous" {
		t.Errorf("contents = %q, want the previous file untouched", got)
	}
}

func TestSaveBytesFileInPlace_NonRetryableOpenFailsFast(t *testing.T) {
	fatal := errors.New("no such volume")
	var calls int
	swapOpenHooks(t,
		func(string) (*os.File, error) { calls++; return nil, fatal },
		func(error) bool { return false },
	)

	if err := SaveBytesFileInPlace([]byte("x"), "unused"); !errors.Is(err, fatal) {
		t.Fatalf("expected the fatal error back, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected a single attempt, got %d", calls)
	}
}
