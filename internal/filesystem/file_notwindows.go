//go:build !windows

package filesystem

// isRetryableRenameError always reports false off Windows. POSIX rename(2) over
// an existing file is atomic and does not fail with the transient sharing
// violations that motivate the retry loop, so a failure here is a real error
// (bad path, cross-device link, permissions) that retrying would not fix.
func isRetryableRenameError(error) bool { return false }
