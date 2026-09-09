//go:build windows

package filesystem

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestSaveJSONFileAtomic_RenameRetriesPastHeldHandle reproduces the Windows
// race the retry loop exists for: another process holds the target file open
// with a share mode that denies rename/delete, so the first os.Rename attempts
// fail with a sharing violation and only succeed once that handle closes.
func TestSaveJSONFileAtomic_RenameRetriesPastHeldHandle(t *testing.T) {
	// Shorten the backoff so the five attempts span a few hundred ms, and make
	// sure the held handle is released partway through that window.
	origBackoff := renameBaseBackoff
	renameBaseBackoff = 20 * time.Millisecond
	t.Cleanup(func() { renameBaseBackoff = origBackoff })

	filename := filepath.Join(t.TempDir(), "record.json")
	if err := SaveJSONFileAtomic(testRecord{Name: "old", Count: 1}, filename); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}

	// Open the target denying write and delete sharing: MoveFileEx (os.Rename's
	// replace path) cannot swap the file while this handle is open.
	pathPtr, err := windows.UTF16PtrFromString(filename)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ, // no FILE_SHARE_WRITE, no FILE_SHARE_DELETE
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	closed := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		windows.CloseHandle(handle)
		close(closed)
	}()

	if err := SaveJSONFileAtomic(testRecord{Name: "new", Count: 2}, filename); err != nil {
		<-closed
		t.Fatalf("SaveJSONFileAtomic should have retried past the held handle, got %v", err)
	}
	<-closed

	var result testRecord
	if err := ReadJSONFile(&result, filename); err != nil {
		t.Fatalf("ReadJSONFile returned error: %v", err)
	}
	if result.Name != "new" || result.Count != 2 {
		t.Errorf("expected the new value to be written, got %+v", result)
	}
}
