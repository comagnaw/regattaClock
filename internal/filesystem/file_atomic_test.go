package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// swapRenameHooks replaces the renameWithRetry seams for one test and restores
// them (and the backoff) afterwards. The backoff is dropped to something
// negligible so retry tests do not spend real time sleeping.
func swapRenameHooks(t *testing.T, rename func(oldPath, newPath string) error, retryable func(error) bool) {
	t.Helper()
	origRename, origRetryable, origBackoff := renameFunc, retryableRename, renameBaseBackoff
	renameFunc = rename
	retryableRename = retryable
	renameBaseBackoff = time.Microsecond
	t.Cleanup(func() {
		renameFunc = origRename
		retryableRename = origRetryable
		renameBaseBackoff = origBackoff
	})
}

func TestSaveJSONFileAtomic(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")
	data := testRecord{Name: "Test Regatta", Count: 3}

	if err := SaveJSONFileAtomic(data, filename); err != nil {
		t.Fatalf("SaveJSONFileAtomic returned error: %v", err)
	}

	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("saved file could not be read: %v", err)
	}

	expected := "{\n  \"name\": \"Test Regatta\",\n  \"count\": 3\n}"
	if string(fileBytes) != expected {
		t.Errorf("Expected file contents %q, got %q", expected, string(fileBytes))
	}
}

func TestSaveJSONFileAtomic_RoundTrip(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "records.json")
	data := []testRecord{{Name: "First", Count: 1}, {Name: "Second", Count: 2}}

	if err := SaveJSONFileAtomic(data, filename); err != nil {
		t.Fatalf("SaveJSONFileAtomic returned error: %v", err)
	}

	var result []testRecord
	if err := ReadJSONFile(&result, filename); err != nil {
		t.Fatalf("ReadJSONFile returned error: %v", err)
	}
	if len(result) != len(data) {
		t.Fatalf("Expected %d records, got %d", len(data), len(result))
	}
	for i, record := range data {
		if result[i] != record {
			t.Errorf("Expected record %d to be %+v, got %+v", i, record, result[i])
		}
	}
}

func TestSaveJSONFileAtomic_OverwritesExistingFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")

	if err := os.WriteFile(filename, []byte("stale contents that are longer than the new ones"), 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	if err := SaveJSONFileAtomic(testRecord{Name: "New", Count: 1}, filename); err != nil {
		t.Fatalf("SaveJSONFileAtomic returned error: %v", err)
	}

	var result testRecord
	if err := ReadJSONFile(&result, filename); err != nil {
		t.Fatalf("ReadJSONFile returned error: %v", err)
	}
	if result.Name != "New" || result.Count != 1 {
		t.Errorf("Expected overwritten record {New 1}, got %+v", result)
	}
}

func TestSaveJSONFileAtomic_NoTempFileLeftBehind(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")

	if err := SaveJSONFileAtomic(testRecord{Name: "Test", Count: 1}, filename); err != nil {
		t.Fatalf("SaveJSONFileAtomic returned error: %v", err)
	}

	if _, err := os.Stat(filename + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("Expected no temp file after a successful write, stat err = %v", err)
	}
}

func TestSaveJSONFileAtomic_MarshalFailureWritesNothing(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")

	// Channels cannot be marshaled to JSON.
	if err := SaveJSONFileAtomic(make(chan int), filename); err == nil {
		t.Fatal("Expected error for data that cannot be marshaled, got nil")
	}

	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Error("No target file should exist when marshaling fails")
	}
	if _, err := os.Stat(filename + ".tmp"); !os.IsNotExist(err) {
		t.Error("No temp file should be left behind when marshaling fails")
	}
}

func TestSaveJSONFileAtomic_RenameFailureRemovesTempAndKeepsOldFile(t *testing.T) {
	swapRenameHooks(t,
		func(_, _ string) error { return errors.New("boom") },
		func(error) bool { return false },
	)

	filename := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(filename, []byte(`{"name":"old","count":9}`), 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	if err := SaveJSONFileAtomic(testRecord{Name: "new", Count: 1}, filename); err == nil {
		t.Fatal("Expected error when rename fails, got nil")
	}

	if _, err := os.Stat(filename + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("Expected temp file to be removed after a failed rename, stat err = %v", err)
	}

	var result testRecord
	if err := ReadJSONFile(&result, filename); err != nil {
		t.Fatalf("ReadJSONFile returned error: %v", err)
	}
	if result.Name != "old" || result.Count != 9 {
		t.Errorf("Expected the original file to be intact, got %+v", result)
	}
}

// TestSaveJSONFileAtomic_ConcurrentReaderSeesWholeFile hammers a file with
// atomic writes while another goroutine reads it, asserting every successful
// read parses cleanly and yields one of the values that was actually written.
func TestSaveJSONFileAtomic_ConcurrentReaderSeesWholeFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")
	if err := SaveJSONFileAtomic(testRecord{Name: "concurrent", Count: 0}, filename); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}

	const writes = 300
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 1; i <= writes; i++ {
			if err := SaveJSONFileAtomic(testRecord{Name: "concurrent", Count: i}, filename); err != nil {
				t.Errorf("write %d failed: %v", i, err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < writes*4; i++ {
			var result testRecord
			if err := ReadJSONFile(&result, filename); err != nil {
				t.Errorf("concurrent read observed a non-parseable file: %v", err)
				return
			}
			if result.Name != "concurrent" || result.Count < 0 || result.Count > writes {
				t.Errorf("concurrent read observed an impossible value: %+v", result)
				return
			}
		}
	}()

	wg.Wait()
}

func TestRenameWithRetry_RetriesThenSucceeds(t *testing.T) {
	retryable := errors.New("held open")
	var calls int
	swapRenameHooks(t,
		func(_, _ string) error {
			calls++
			if calls < 3 {
				return retryable
			}
			return nil
		},
		func(err error) bool { return errors.Is(err, retryable) },
	)

	if err := renameWithRetry("old", "new"); err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestRenameWithRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	retryable := errors.New("still held")
	var calls int
	swapRenameHooks(t,
		func(_, _ string) error { calls++; return retryable },
		func(err error) bool { return errors.Is(err, retryable) },
	)

	err := renameWithRetry("old", "new")
	if !errors.Is(err, retryable) {
		t.Fatalf("expected the retryable error back, got %v", err)
	}
	if calls != renameMaxAttempts {
		t.Fatalf("expected exactly %d attempts, got %d", renameMaxAttempts, calls)
	}
}

func TestRenameWithRetry_NonRetryableErrorFailsFast(t *testing.T) {
	fatal := errors.New("cross-device link")
	var calls int
	swapRenameHooks(t,
		func(_, _ string) error { calls++; return fatal },
		func(error) bool { return false },
	)

	if err := renameWithRetry("old", "new"); !errors.Is(err, fatal) {
		t.Fatalf("expected the fatal error back, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected a single attempt for a non-retryable error, got %d", calls)
	}
}

func TestHashBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "hello world",
			input:    "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HashBytes([]byte(tt.input)); got != tt.expected {
				t.Errorf("HashBytes(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHashBytes_MatchesFileHash(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "hashed.txt")
	contents := []byte(`{"name":"Head of the Charles","count":42}`)
	if err := os.WriteFile(filename, contents, 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	fromFile, err := FileHash(filename)
	if err != nil {
		t.Fatalf("FileHash returned error: %v", err)
	}
	if fromBytes := HashBytes(contents); fromBytes != fromFile {
		t.Errorf("HashBytes = %q, FileHash = %q, expected equal", fromBytes, fromFile)
	}
}
