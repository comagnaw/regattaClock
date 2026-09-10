package uitheme

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestThemedColor_PrimaryIsAccentOnBothVariants(t *testing.T) {
	for _, v := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		got := Themed(v).Color(theme.ColorNamePrimary, v)
		if got != color.Color(LogoWaterBlue) {
			t.Errorf("variant %d primary = %v, want LogoWaterBlue", v, got)
		}
	}
}

func TestThemedColor_DarkRetintsSurfaces(t *testing.T) {
	dark := Themed(theme.VariantDark)

	if got := dark.Color(theme.ColorNameBackground, theme.VariantDark); got != color.Color(BrandNavy) {
		t.Errorf("dark background = %v, want BrandNavy", got)
	}

	// A surface that is retinted goes through shade(); assert it changed from the
	// default theme's neutral grey and stayed a shade of navy (blue dominant).
	def := theme.DefaultTheme().Color(theme.ColorNameButton, theme.VariantDark)
	got := dark.Color(theme.ColorNameButton, theme.VariantDark)
	if got == def {
		t.Fatalf("dark button colour was not retinted: %v", got)
	}
	r, g, b, _ := got.RGBA()
	if b <= r || b <= g {
		t.Errorf("retinted button %v is not blue-dominant (r=%d g=%d b=%d)", got, r, g, b)
	}

	// Error stays semantic (not in darkSurfaces), so it is untouched.
	if got, want := dark.Color(theme.ColorNameError, theme.VariantDark),
		theme.DefaultTheme().Color(theme.ColorNameError, theme.VariantDark); got != want {
		t.Errorf("dark error = %v, want the default %v (semantic colours are not retinted)", got, want)
	}
}

func TestThemedColor_LightDefersToDefault(t *testing.T) {
	light := Themed(theme.VariantLight)
	got := light.Color(theme.ColorNameBackground, theme.VariantLight)
	want := theme.DefaultTheme().Color(theme.ColorNameBackground, theme.VariantLight)
	if got != want {
		t.Errorf("light background = %v, want the default %v (surfaces retint in dark only)", got, want)
	}
}

func TestReverseCardColors_SwapByVariant(t *testing.T) {
	card, ink := ReverseCardColors(theme.VariantLight)
	if card != color.Color(BrandNavy) || ink != color.Color(White) {
		t.Errorf("light card/ink = %v/%v, want navy/white", card, ink)
	}
	card, ink = ReverseCardColors(theme.VariantDark)
	if card != color.Color(White) || ink != color.Color(BrandNavy) {
		t.Errorf("dark card/ink = %v/%v, want white/navy", card, ink)
	}
}

func TestRule_SetsMinHeight(t *testing.T) {
	r := Rule(3, theme.ColorNameForeground)
	if h := r.MinSize().Height; h != 3 {
		t.Errorf("rule min height = %v, want 3", h)
	}
}

func TestColor_ResolvesAgainstLiveTheme(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(Themed(theme.VariantDark))

	if got := Color(theme.ColorNamePrimary); got != color.Color(LogoWaterBlue) {
		t.Errorf("Color(primary) = %v, want LogoWaterBlue", got)
	}
}

func TestFramingHelpers_Build(t *testing.T) {
	test.NewApp() // a fyne app must exist for theme.Padding()

	if c := FullBleed(Rule(2, theme.ColorNameSeparator)); c == nil || len(c.Objects) != 1 {
		t.Fatal("FullBleed should wrap exactly one object")
	}
	if b := AccentBand(Rule(1, theme.ColorNameForeground), 4); b == nil {
		t.Fatal("AccentBand returned nil")
	}
	strip := CautionStrip(Rule(1, theme.ColorNameForeground))
	if strip == nil || strip.Hidden {
		t.Fatal("CautionStrip should return a visible container")
	}
}
