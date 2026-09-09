package watcher

import (
	"testing"

	"github.com/comagnaw/regattaClock/internal/common"
)

func TestParseMode(t *testing.T) {
	cases := map[string]Mode{
		"smb":      ModeSMB,
		"SMB":      ModeSMB,
		"  smb  ":  ModeSMB,
		"cloud":    ModeCloud,
		"":         ModeCloud,
		"onedrive": ModeCloud,
		"nas":      ModeCloud,
	}
	for in, want := range cases {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestModeValuesMatchCommon(t *testing.T) {
	if string(ModeCloud) != common.StorageModeCloud {
		t.Errorf("ModeCloud %q != common.StorageModeCloud %q", ModeCloud, common.StorageModeCloud)
	}
	if string(ModeSMB) != common.StorageModeSMB {
		t.Errorf("ModeSMB %q != common.StorageModeSMB %q", ModeSMB, common.StorageModeSMB)
	}
}
