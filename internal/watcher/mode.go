package watcher

import "strings"

// ParseMode maps a stored PrefStorageMode value to a Mode. Only an exact "smb"
// (trimmed, any case) selects ModeSMB; everything else - including the empty
// string and unrecognised values - is ModeCloud, the safe default that also
// works for a local folder that is not actually synced.
func ParseMode(s string) Mode {
	if strings.EqualFold(strings.TrimSpace(s), string(ModeSMB)) {
		return ModeSMB
	}
	return ModeCloud
}
