package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/comagnaw/regattaClock/internal/common"
)

type testRecord struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestSaveJSONFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")
	data := testRecord{Name: "Test Regatta", Count: 3}

	if err := SaveJSONFile(data, filename); err != nil {
		t.Fatalf("SaveJSONFile returned error: %v", err)
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

func TestSaveJSONFile_OverwritesExistingFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")

	if err := os.WriteFile(filename, []byte("stale contents that are longer than the new ones"), 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	if err := SaveJSONFile(testRecord{Name: "New", Count: 1}, filename); err != nil {
		t.Fatalf("SaveJSONFile returned error: %v", err)
	}

	var result testRecord
	if err := ReadJSONFile(&result, filename); err != nil {
		t.Fatalf("ReadJSONFile returned error: %v", err)
	}

	if result.Name != "New" || result.Count != 1 {
		t.Errorf("Expected overwritten record {New 1}, got %+v", result)
	}
}

func TestSaveJSONFile_UnmarshalableData(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")

	// Channels cannot be marshaled to JSON.
	if err := SaveJSONFile(make(chan int), filename); err == nil {
		t.Fatal("Expected error for data that cannot be marshaled, got nil")
	}

	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Error("No file should be written when marshaling fails")
	}
}

func TestSaveJSONFile_UnwritablePath(t *testing.T) {
	// The parent directory does not exist, so the write must fail.
	filename := filepath.Join(t.TempDir(), "missing", "record.json")

	if err := SaveJSONFile(testRecord{Name: "Test", Count: 1}, filename); err == nil {
		t.Fatal("Expected error for a path that cannot be written, got nil")
	}
}

func TestReadJSONFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")

	if err := os.WriteFile(filename, []byte(`{"name":"Head of the Charles","count":42}`), 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	var result testRecord
	if err := ReadJSONFile(&result, filename); err != nil {
		t.Fatalf("ReadJSONFile returned error: %v", err)
	}

	if result.Name != "Head of the Charles" {
		t.Errorf("Expected name %q, got %q", "Head of the Charles", result.Name)
	}

	if result.Count != 42 {
		t.Errorf("Expected count 42, got %d", result.Count)
	}
}

func TestReadJSONFile_RoundTrip(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "records.json")
	data := []testRecord{{Name: "First", Count: 1}, {Name: "Second", Count: 2}}

	if err := SaveJSONFile(data, filename); err != nil {
		t.Fatalf("SaveJSONFile returned error: %v", err)
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

func TestReadJSONFile_MissingFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "does-not-exist.json")

	var result testRecord
	if err := ReadJSONFile(&result, filename); err == nil {
		t.Fatal("Expected error for a missing file, got nil")
	}
}

func TestReadJSONFile_InvalidJSON(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "record.json")

	if err := os.WriteFile(filename, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	var result testRecord
	if err := ReadJSONFile(&result, filename); err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestReadJSONFile_EmptyFilename(t *testing.T) {
	var result testRecord
	if err := ReadJSONFile(&result, common.EmptyString); err == nil {
		t.Fatal("Expected error for an empty filename, got nil")
	}
}

func TestFileHash(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		expected string
	}{
		{
			name:     "empty file",
			contents: common.EmptyString,
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "populated file",
			contents: "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "hashed.txt")
			if err := os.WriteFile(filename, []byte(tt.contents), 0644); err != nil {
				t.Fatalf("test file could not be seeded: %v", err)
			}

			hash, err := FileHash(filename)
			if err != nil {
				t.Fatalf("FileHash returned error: %v", err)
			}

			if hash != tt.expected {
				t.Errorf("Expected hash %q, got %q", tt.expected, hash)
			}
		})
	}
}

func TestFileHash_IdenticalContentsMatch(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	third := filepath.Join(dir, "third.json")

	if err := SaveJSONFile(testRecord{Name: "Test", Count: 1}, first); err != nil {
		t.Fatalf("SaveJSONFile returned error: %v", err)
	}
	if err := SaveJSONFile(testRecord{Name: "Test", Count: 1}, second); err != nil {
		t.Fatalf("SaveJSONFile returned error: %v", err)
	}
	if err := SaveJSONFile(testRecord{Name: "Test", Count: 2}, third); err != nil {
		t.Fatalf("SaveJSONFile returned error: %v", err)
	}

	firstHash, err := FileHash(first)
	if err != nil {
		t.Fatalf("FileHash returned error: %v", err)
	}
	secondHash, err := FileHash(second)
	if err != nil {
		t.Fatalf("FileHash returned error: %v", err)
	}
	thirdHash, err := FileHash(third)
	if err != nil {
		t.Fatalf("FileHash returned error: %v", err)
	}

	if firstHash != secondHash {
		t.Errorf("Expected identical files to hash the same, got %q and %q", firstHash, secondHash)
	}

	if firstHash == thirdHash {
		t.Error("Expected differing files to hash differently")
	}
}

func TestFileHash_MissingFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "does-not-exist.txt")

	hash, err := FileHash(filename)
	if err == nil {
		t.Fatal("Expected error for a missing file, got nil")
	}

	if hash != common.EmptyString {
		t.Errorf("Expected empty hash on error, got %q", hash)
	}
}

func TestFileHash_Directory(t *testing.T) {
	hash, err := FileHash(t.TempDir())
	if err == nil {
		t.Fatal("Expected error when hashing a directory, got nil")
	}

	if hash != common.EmptyString {
		t.Errorf("Expected empty hash on error, got %q", hash)
	}
}
