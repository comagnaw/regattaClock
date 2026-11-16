package reader

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf16"
)

// ownerFileMaxSize bounds what ExcelLockOwner will read. Excel's owner file is
// 165 bytes; anything much larger is not one.
const ownerFileMaxSize = 4096

// ExcelLockOwner returns the name of the user Excel records as having
// workbookPath open, read from Excel's owner file ("~$<name>", hidden, beside
// the workbook) - the same file behind Excel's own "locked for editing by"
// prompt. ok is false when there is no owner file or it does not parse.
//
// The format is undocumented, so this is best-effort and callers must have a
// generic fallback. Observed layout: byte 0 is the length n of an ANSI user
// name in bytes 1..n (space-padded to 54 bytes); at offset 54 a little-endian
// uint16 length m precedes the same name as m UTF-16LE units. The UTF-16 copy
// is preferred (it survives non-Latin names). An owner file is only present
// where Excel opened the workbook from - a OneDrive client does not sync it -
// so on a cloud-synced folder this only ever names someone on this computer.
func ExcelLockOwner(workbookPath string) (name string, ok bool) {
	dir, base := filepath.Split(workbookPath)
	candidates := []string{"~$" + base}
	if r := []rune(base); len(r) > 2 {
		// Word-style naming replaces the first two characters instead of
		// prefixing; accept it too in case Excel does the same for long names.
		candidates = append(candidates, "~$"+string(r[2:]))
	}
	for _, c := range candidates {
		b, err := readSmall(filepath.Join(dir, c))
		if err != nil {
			continue
		}
		if n, ok := parseOwnerFile(b); ok {
			return n, true
		}
	}
	return "", false
}

func readSmall(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, ownerFileMaxSize)
	n, err := f.Read(buf)
	if n == 0 && err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// parseOwnerFile extracts the user name from an owner file's bytes, preferring
// the UTF-16 copy at offset 54 and falling back to the length-prefixed ANSI one.
func parseOwnerFile(b []byte) (string, bool) {
	const utf16At = 54
	if len(b) >= utf16At+2 {
		m := int(binary.LittleEndian.Uint16(b[utf16At:]))
		end := utf16At + 2 + 2*m
		if m > 0 && end <= len(b) {
			units := make([]uint16, m)
			for i := range units {
				units[i] = binary.LittleEndian.Uint16(b[utf16At+2+2*i:])
			}
			if name, ok := cleanOwnerName(string(utf16.Decode(units))); ok {
				return name, true
			}
		}
	}
	if len(b) >= 1 {
		n := int(b[0])
		if n > 0 && 1+n <= len(b) {
			if name, ok := cleanOwnerName(string(b[1 : 1+n])); ok {
				return name, true
			}
		}
	}
	return "", false
}

// cleanOwnerName trims padding and rejects anything that is not a plausible
// display name, so a misparse falls back to generic text rather than showing
// garbage in a banner.
func cleanOwnerName(s string) (string, bool) {
	s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == 0 })
	if s == "" || len([]rune(s)) > 100 {
		return "", false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return "", false
		}
	}
	return s, true
}
