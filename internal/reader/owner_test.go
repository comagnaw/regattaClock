package reader

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

// ownerFileBytes builds an owner file in Excel's observed 165-byte layout.
func ownerFileBytes(ansi, wide string) []byte {
	b := make([]byte, 165)
	for i := 1; i < 54; i++ {
		b[i] = ' '
	}
	b[0] = byte(len(ansi))
	copy(b[1:], ansi)
	units := utf16.Encode([]rune(wide))
	binary.LittleEndian.PutUint16(b[54:], uint16(len(units)))
	for i, u := range units {
		binary.LittleEndian.PutUint16(b[56+2*i:], u)
	}
	return b
}

func writeOwner(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExcelLockOwner_ReadsName(t *testing.T) {
	dir := t.TempDir()
	writeOwner(t, dir, "~$Heat Sheet.xlsm", ownerFileBytes("Jane Smith", "Jane Smith"))

	got, ok := ExcelLockOwner(filepath.Join(dir, "Heat Sheet.xlsm"))
	if !ok || got != "Jane Smith" {
		t.Errorf("ExcelLockOwner = %q, %v; want %q, true", got, ok, "Jane Smith")
	}
}

func TestExcelLockOwner_PrefersUTF16(t *testing.T) {
	dir := t.TempDir()
	writeOwner(t, dir, "~$w.xlsx", ownerFileBytes("Ren? Lef?vre", "René Lefèvre"))

	if got, _ := ExcelLockOwner(filepath.Join(dir, "w.xlsx")); got != "René Lefèvre" {
		t.Errorf("ExcelLockOwner = %q, want the UTF-16 name", got)
	}
}

func TestExcelLockOwner_FallsBackToANSI(t *testing.T) {
	dir := t.TempDir()
	b := ownerFileBytes("Jane Smith", "")
	writeOwner(t, dir, "~$w.xlsx", b[:54]) // truncated before the UTF-16 copy

	if got, ok := ExcelLockOwner(filepath.Join(dir, "w.xlsx")); !ok || got != "Jane Smith" {
		t.Errorf("ExcelLockOwner = %q, %v; want the ANSI name", got, ok)
	}
}

func TestExcelLockOwner_WordStyleName(t *testing.T) {
	dir := t.TempDir()
	writeOwner(t, dir, "~$at Sheet.xlsm", ownerFileBytes("Jane", "Jane"))

	if got, ok := ExcelLockOwner(filepath.Join(dir, "Heat Sheet.xlsm")); !ok || got != "Jane" {
		t.Errorf("ExcelLockOwner = %q, %v; want the first-two-chars-replaced owner file found", got, ok)
	}
}

func TestExcelLockOwner_MissingOrGarbage(t *testing.T) {
	dir := t.TempDir()
	if _, ok := ExcelLockOwner(filepath.Join(dir, "none.xlsx")); ok {
		t.Error("no owner file should report ok=false")
	}

	writeOwner(t, dir, "~$bad.xlsx", []byte{0x05, 0x01, 0x02, 0x03, 0x04, 0x05})
	if got, ok := ExcelLockOwner(filepath.Join(dir, "bad.xlsx")); ok {
		t.Errorf("unprintable owner file parsed as %q, want ok=false", got)
	}
}
