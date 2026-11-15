//go:build windows

package filesystem

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// holdDenyingWrite opens filename the way Excel holds an open workbook -
// readers allowed, writers and deleters refused - and returns the handle.
func holdDenyingWrite(t *testing.T, filename string) windows.Handle {
	t.Helper()
	pathPtr, err := windows.UTF16PtrFromString(filename)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	h, err := windows.CreateFile(pathPtr, windows.GENERIC_READ, windows.FILE_SHARE_READ,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	return h
}

// TestSaveBytesFileInPlace_HeldOpenIsLocked: a file held open for the whole
// retry window (Excel) fails with an error IsFileLocked recognises, and the
// previous contents are untouched.
func TestSaveBytesFileInPlace_HeldOpenIsLocked(t *testing.T) {
	origBackoff := renameBaseBackoff
	renameBaseBackoff = time.Millisecond
	t.Cleanup(func() { renameBaseBackoff = origBackoff })

	filename := filepath.Join(t.TempDir(), "results.xlsx")
	if err := os.WriteFile(filename, []byte("previous"), 0644); err != nil {
		t.Fatal(err)
	}
	h := holdDenyingWrite(t, filename)
	defer windows.CloseHandle(h)

	err := SaveBytesFileInPlace([]byte("new"), filename)
	if err == nil || !IsFileLocked(err) {
		t.Fatalf("SaveBytesFileInPlace(held) error = %v, want a locked-file error", err)
	}
	windows.CloseHandle(h)
	if got, _ := os.ReadFile(filename); string(got) != "previous" {
		t.Errorf("contents = %q, want the previous file untouched", got)
	}
}

// TestSaveBytesFileInPlace_RetriesPastBriefHold: a handle released partway
// through the retry window (a sync client or Defender) is waited out.
func TestSaveBytesFileInPlace_RetriesPastBriefHold(t *testing.T) {
	origBackoff := renameBaseBackoff
	renameBaseBackoff = 20 * time.Millisecond
	t.Cleanup(func() { renameBaseBackoff = origBackoff })

	filename := filepath.Join(t.TempDir(), "results.xlsx")
	if err := os.WriteFile(filename, []byte("previous"), 0644); err != nil {
		t.Fatal(err)
	}
	h := holdDenyingWrite(t, filename)
	closed := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		windows.CloseHandle(h)
		close(closed)
	}()

	err := SaveBytesFileInPlace([]byte("new"), filename)
	<-closed
	if err != nil {
		t.Fatalf("SaveBytesFileInPlace should have retried past the brief hold, got %v", err)
	}
	if got, _ := os.ReadFile(filename); string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}
}
