package filesystem

import (
	"regexp"
	"strings"
)

// unsafeFilenameChars matches characters that are illegal in a Windows filename
// (< > : " / \ | ? *) plus ASCII control characters. POSIX only forbids "/" and
// NUL, so this is the stricter of the two rule sets.
var unsafeFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// reservedDeviceNames are Windows reserved device names. A file whose stem (the
// part before the first ".") matches one of these, case-insensitively, cannot be
// created on Windows regardless of extension.
var reservedDeviceNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {},
	"COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {},
	"LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

// SanitizeForFilename reduces name to a single path component that is safe to
// write on Windows as well as POSIX. Reserved and control characters become "_",
// trailing spaces and dots are trimmed (Windows silently strips them), and a
// name whose stem is a Windows reserved device name is prefixed with "_". An
// empty or fully-stripped result becomes "_" so callers always get a usable
// component. Spaces are left alone; they are legal on both platforms.
func SanitizeForFilename(name string) string {
	out := unsafeFilenameChars.ReplaceAllString(name, "_")
	out = strings.TrimRight(out, " .")
	if out == "" {
		return "_"
	}

	stem := out
	if i := strings.IndexByte(out, '.'); i > 0 {
		stem = out[:i]
	}
	if _, reserved := reservedDeviceNames[strings.ToUpper(stem)]; reserved {
		out = "_" + out
	}
	return out
}
