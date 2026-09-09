package timesync

import "strings"

// ParseServers splits a stored PrefNTPServers value ("host1, host2, …") into a
// server list, trimming each entry and dropping blanks. A blank or all-blank
// input returns nil, so Config falls back to DefaultServers. Under smb mode the
// operator is expected to put the LAN NTP host first.
func ParseServers(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if host := strings.TrimSpace(part); host != "" {
			out = append(out, host)
		}
	}
	return out
}
