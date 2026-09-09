//go:build windows

package filesystem

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isRetryableRenameError reports whether a failed os.Rename is worth retrying.
// On Windows the target of the rename can be briefly held open by a cloud-sync
// client, Windows Defender, or an SMB peer that has just noticed the file
// change; both cases surface as ERROR_SHARING_VIOLATION (32) or
// ERROR_ACCESS_DENIED (5) and clear within milliseconds.
func isRetryableRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
