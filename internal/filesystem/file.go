package filesystem

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/comagnaw/regattaClock/internal/common"
)

// renameMaxAttempts bounds the retry loop in renameWithRetry. On Windows a
// cloud-sync client, Defender, or an SMB peer can hold a transient handle on a
// file it has just noticed change, so os.Rename can fail with a sharing
// violation that clears within milliseconds. Five attempts with a backoff
// doubling from renameBaseBackoff (50, 100, 200, 400ms of waiting) covers that
// window without stalling a caller for long when the target is genuinely stuck.
const renameMaxAttempts = 5

// renameBaseBackoff, renameFunc, and retryableRename are indirection seams so
// the attempt/backoff logic in renameWithRetry can be unit-tested on any OS.
// Production code always uses os.Rename and the build-tagged
// isRetryableRenameError.
var (
	renameBaseBackoff = 50 * time.Millisecond
	renameFunc        = os.Rename
	retryableRename   = isRetryableRenameError
)

func SaveJSONFile(data interface{}, filename string) error {
	fileBytes, err := json.MarshalIndent(data, common.EmptyString, "  ")
	if err != nil {
		return fmt.Errorf("data could not be marshaled into filename %s:%s", filename, err)
	}

	err = os.WriteFile(filename, fileBytes, 0644)
	if err != nil {
		return fmt.Errorf("filename %s could not be written: %w", filename, err)
	}
	return nil
}

// SaveJSONFileAtomic marshals data and writes it to a sibling temp file, fsyncs
// and closes it, then renames it over filename. A reader that opens filename
// while the write is in progress sees either the whole previous file or the
// whole new one, never a truncated document. The rename is retried because on
// Windows another process may briefly hold the target open (see
// renameWithRetry). The temp file is a sibling of the target so the rename stays
// within one filesystem, where it is atomic; it is removed on any failure.
func SaveJSONFileAtomic(data any, filename string) error {
	fileBytes, err := json.MarshalIndent(data, common.EmptyString, "  ")
	if err != nil {
		return fmt.Errorf("data could not be marshaled into filename %s: %w", filename, err)
	}

	tmp := filename + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("temp file %s could not be created: %w", tmp, err)
	}

	if _, err = f.Write(fileBytes); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("temp file %s could not be written: %w", tmp, err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("temp file %s could not be synced: %w", tmp, err)
	}
	if err = f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("temp file %s could not be closed: %w", tmp, err)
	}

	if err = renameWithRetry(tmp, filename); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("temp file %s could not replace %s: %w", tmp, filename, err)
	}
	return nil
}

// renameWithRetry renames oldPath to newPath, retrying transient failures that
// isRetryableRenameError recognises (Windows sharing violations and access-denied
// races from sync clients, Defender, or SMB peers). A non-retryable error, or the
// final attempt, is returned immediately. On non-Windows platforms
// isRetryableRenameError always returns false, so this is a single os.Rename.
func renameWithRetry(oldPath, newPath string) error {
	backoff := renameBaseBackoff
	var err error
	for attempt := 1; attempt <= renameMaxAttempts; attempt++ {
		if err = renameFunc(oldPath, newPath); err == nil {
			return nil
		}
		if attempt == renameMaxAttempts || !retryableRename(err) {
			return err
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return err
}

func ReadJSONFile(data interface{}, filename string) error {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("filename %s could not be read: %w", filename, err)
	}
	err = json.Unmarshal(fileBytes, data)
	if err != nil {
		return fmt.Errorf("filename %s could not be unmarshalled: %s", filename, err)
	}
	return nil
}

// HashBytes returns the hex-encoded SHA-256 of b. It is the in-memory
// counterpart to FileHash, for callers that already hold the file contents and
// want to know whether they changed without a second read (e.g. a watcher's
// stat-then-hash short circuit).
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func FileHash(filename string) (string, error) {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("filename %s could not be read: %s", filename, err)
	}
	return HashBytes(fileBytes), nil
}
