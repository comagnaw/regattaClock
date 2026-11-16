package filesystem

import (
	"fmt"
	"io"
	"os"
	"time"
)

// openForInPlaceFunc is an indirection seam so the open retry in
// SaveBytesFileInPlace can be unit-tested on any OS. Production code always
// opens through os.OpenFile.
var openForInPlaceFunc = func(filename string) (*os.File, error) {
	return os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
}

// SaveBytesFileInPlace overwrites filename's contents with fileBytes through a
// handle on the file itself, rather than writing a sibling temp file and
// renaming it over the target the way SaveBytesFileAtomic does.
//
// It exists for files a cloud-sync client and Office co-own - the published
// results workbook. A rename-over replaces the file's identity: OneDrive
// tracks items by file ID, not name, so it sees the original deleted and a
// stranger (born as an unsynced "*.tmp") take its name. It may never upload
// that as a new version, and Excel - which opens a OneDrive file as the cloud
// document - then reports the local copy as "unmerged" and offers the stale
// server copy instead. Writing in place keeps the file ID, so the sync client
// sees an ordinary edit and uploads it as the next version.
//
// What in-place gives up is atomicity, so it compensates: the previous bytes
// are read through the same handle first and written back if the new write
// fails partway, leaving the file as it was rather than truncated. A reader
// that opens the file mid-write can still see a partial document; use
// SaveBytesFileAtomic for files other processes read live.
//
// Opening is retried like renameWithRetry: on Windows, Excel holding the file
// open, or a sync client / Defender briefly holding it, fails the open with a
// sharing violation. A persistent one is returned, and IsFileLocked
// recognises it.
func SaveBytesFileInPlace(fileBytes []byte, filename string) error {
	f, err := openWithRetry(filename)
	if err != nil {
		return fmt.Errorf("file %s could not be opened for writing: %w", filename, err)
	}

	prev, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		return fmt.Errorf("file %s could not be read before rewriting: %w", filename, err)
	}

	if err := overwrite(f, fileBytes); err != nil {
		restoreErr := overwrite(f, prev)
		f.Close()
		if restoreErr != nil {
			return fmt.Errorf("file %s could not be written (%w) and its previous contents could not be restored: %w", filename, err, restoreErr)
		}
		return fmt.Errorf("file %s could not be written, previous contents kept: %w", filename, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("file %s could not be closed: %w", filename, err)
	}
	return nil
}

// overwrite replaces everything in f with b and flushes it to disk.
func overwrite(f *os.File, b []byte) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Truncate(int64(len(b))); err != nil {
		return err
	}
	return f.Sync()
}

// openWithRetry opens filename for SaveBytesFileInPlace, retrying the same
// transient Windows sharing failures renameWithRetry does, with the same
// attempt count and backoff.
func openWithRetry(filename string) (*os.File, error) {
	backoff := renameBaseBackoff
	var err error
	for attempt := 1; attempt <= renameMaxAttempts; attempt++ {
		var f *os.File
		if f, err = openForInPlaceFunc(filename); err == nil {
			return f, nil
		}
		if attempt == renameMaxAttempts || !retryableRename(err) {
			return nil, err
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return nil, err
}
