package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/comagnaw/regattaClock/internal/common"
)

func TestCreateDirs(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "regatta", "2024", "results")

	if err := CreateDirs(dirPath); err != nil {
		t.Fatalf("CreateDirs returned error: %v", err)
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("created directory could not be stat'd: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dirPath)
	}
}

func TestCreateDirs_ExistingDir(t *testing.T) {
	dirPath := t.TempDir()

	// Creating an existing directory is a no-op rather than an error.
	if err := CreateDirs(dirPath); err != nil {
		t.Fatalf("CreateDirs returned error for an existing directory: %v", err)
	}

	if !DirExists(dirPath) {
		t.Errorf("Expected %s to still exist", dirPath)
	}
}

func TestCreateDirs_EmptyPath(t *testing.T) {
	if err := CreateDirs(common.EmptyString); err == nil {
		t.Fatal("Expected error for an empty directory path, got nil")
	}
}

func TestCreateDirs_PathBlockedByFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(filename, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	dirPath := filepath.Join(filename, "child")

	// Stat fails with ENOTDIR rather than ENOENT here, so DirExists reports
	// true and CreateDirs reports success without creating anything.
	if err := CreateDirs(dirPath); err != nil {
		t.Fatalf("CreateDirs returned error: %v", err)
	}

	if _, err := os.Stat(dirPath); err == nil {
		t.Errorf("Expected %s not to exist", dirPath)
	}
}

func TestCreateDirs_UnwritableParent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	parent := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(parent, 0555); err != nil {
		t.Fatalf("test directory could not be seeded: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(parent, 0755)
	})

	if err := CreateDirs(filepath.Join(parent, "child")); err == nil {
		t.Fatal("Expected error when the parent directory is not writable, got nil")
	}
}

func TestDirExists(t *testing.T) {
	dirPath := t.TempDir()
	filename := filepath.Join(dirPath, "file.json")
	if err := os.WriteFile(filename, []byte("{}"), 0644); err != nil {
		t.Fatalf("test file could not be seeded: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{name: "existing directory", path: dirPath, expected: true},
		{name: "missing directory", path: filepath.Join(dirPath, "missing"), expected: false},
		{name: "empty path", path: common.EmptyString, expected: false},
		// DirExists only checks for existence, so a file also reports true.
		{name: "existing file", path: filename, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DirExists(tt.path); got != tt.expected {
				t.Errorf("Expected DirExists(%q) to be %t, got %t", tt.path, tt.expected, got)
			}
		})
	}
}

func TestReadDir(t *testing.T) {
	dirPath := t.TempDir()
	seedFiles(t, dirPath, "alpha.json", "beta.json")

	if err := os.Mkdir(filepath.Join(dirPath, "nested"), 0755); err != nil {
		t.Fatalf("test directory could not be seeded: %v", err)
	}

	listing, err := ReadDir(dirPath)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}

	if len(listing) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(listing))
	}

	// os.ReadDir sorts entries by filename.
	expectedNames := []string{"alpha.json", "beta.json", "nested"}
	for i, name := range expectedNames {
		if listing[i].Name() != name {
			t.Errorf("Expected entry %d to be %q, got %q", i, name, listing[i].Name())
		}
	}
}

func TestReadDir_EmptyDir(t *testing.T) {
	listing, err := ReadDir(t.TempDir())
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}

	if len(listing) != 0 {
		t.Errorf("Expected no entries, got %d", len(listing))
	}
}

func TestReadDir_MissingDir(t *testing.T) {
	listing, err := ReadDir(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("Expected error for a missing directory, got nil")
	}

	if listing != nil {
		t.Errorf("Expected nil listing on error, got %v", listing)
	}
}

func TestReadDir_EmptyPath(t *testing.T) {
	if _, err := ReadDir(common.EmptyString); err == nil {
		t.Fatal("Expected error for an empty directory path, got nil")
	}
}

func TestGetFilteredFilesInDir(t *testing.T) {
	dirPath := t.TempDir()
	seedFiles(t, dirPath,
		"regatta-2024.json",
		"regatta-2025.json",
		"schools.xlsx",
		"theme.json",
	)

	tests := []struct {
		name     string
		subStr   string
		expected []string
	}{
		{
			name:     "matches a subset",
			subStr:   "regatta",
			expected: []string{"regatta-2024.json", "regatta-2025.json"},
		},
		{
			name:     "matches by extension",
			subStr:   ".json",
			expected: []string{"regatta-2024.json", "regatta-2025.json", "theme.json"},
		},
		{
			name:     "matches a single file",
			subStr:   "xlsx",
			expected: []string{"schools.xlsx"},
		},
		{
			name:     "no matches",
			subStr:   "no-such-file",
			expected: []string{},
		},
		{
			name:     "empty substring matches everything",
			subStr:   common.EmptyString,
			expected: []string{"regatta-2024.json", "regatta-2025.json", "schools.xlsx", "theme.json"},
		},
		{
			name:     "filter is case sensitive",
			subStr:   "REGATTA",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetFilteredFilesInDir(dirPath, tt.subStr)
			if err != nil {
				t.Fatalf("GetFilteredFilesInDir returned error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d entries, got %d", len(tt.expected), len(result))
			}

			for i, name := range tt.expected {
				if result[i].Name() != name {
					t.Errorf("Expected entry %d to be %q, got %q", i, name, result[i].Name())
				}
			}
		})
	}
}

func TestGetFilteredFilesInDir_IncludesDirs(t *testing.T) {
	dirPath := t.TempDir()
	seedFiles(t, dirPath, "regatta.json")

	if err := os.Mkdir(filepath.Join(dirPath, "regatta-archive"), 0755); err != nil {
		t.Fatalf("test directory could not be seeded: %v", err)
	}

	// The filter matches on name only, so directories are returned too.
	result, err := GetFilteredFilesInDir(dirPath, "regatta")
	if err != nil {
		t.Fatalf("GetFilteredFilesInDir returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(result))
	}
}

func TestGetFilteredFilesInDir_MissingDir(t *testing.T) {
	result, err := GetFilteredFilesInDir(filepath.Join(t.TempDir(), "missing"), "regatta")
	if err == nil {
		t.Fatal("Expected error for a missing directory, got nil")
	}

	if result == nil {
		t.Fatal("Expected an empty slice on error, got nil")
	}

	if len(result) != 0 {
		t.Errorf("Expected no entries on error, got %d", len(result))
	}
}

func seedFiles(t *testing.T, dirPath string, filenames ...string) {
	t.Helper()

	for _, name := range filenames {
		if err := os.WriteFile(filepath.Join(dirPath, name), []byte("{}"), 0644); err != nil {
			t.Fatalf("test file %s could not be seeded: %v", name, err)
		}
	}
}
