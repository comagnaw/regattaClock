// Package uitheme holds the app's brand theme and the small set of framing
// helpers (full-bleed bands, rules, reverse-contrast cards, caution strips) that
// depend on the brand palette. It is shared by every window package so the race
// tree, the clock, and any future surface read as one product.
package uitheme

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

var (
	// BrandNavy - the deep blue of the branding banner, used as the window
	// background. The exact brand value rather than a shade derived from it, so
	// the background matches the artwork instead of merely approximating it.
	BrandNavy = color.NRGBA{R: 33, G: 53, B: 76, A: 0xFF}

	// LogoWaterBlue - the mid tone that covered most of the water in the original
	// banner artwork, kept as the accent color.
	LogoWaterBlue = color.NRGBA{R: 0x05, G: 0x69, B: 0xA6, A: 0xFF}

	// BannerAmber - caution fill for notice strips (clock-skew, staleness,
	// stale-lane-map, schedule-conflict, origin-change), so they stand out above
	// the content they sit over. Paired with LightOverride for dark text on the
	// tint in both app themes.
	BannerAmber = color.NRGBA{R: 0xF2, G: 0xC7, B: 0x44, A: 0xFF}

	// White - plain white, the reverse-contrast card fill / ink on the dark and
	// light themes respectively.
	White = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// Theme is the app's brand theme: the primary accent is LogoWaterBlue on both
// variants, and in dark mode every neutral grey surface Fyne would draw is
// restated as a shade of BrandNavy. Construct with Themed.
type Theme struct {
	fyne.Theme

	variant fyne.ThemeVariant
}

// Themed returns the brand Theme pinned to variant.
func Themed(variant fyne.ThemeVariant) *Theme {
	return &Theme{Theme: theme.DefaultTheme(), variant: variant}
}

// LightOverride / DarkOverride - fixed-variant brand themes for the
// container.NewThemeOverride wrappers that must render one way regardless of the
// app's current theme choice (a caution strip's dark text on amber; a coloured
// band's light text on blue).
var (
	LightOverride = Themed(theme.VariantLight)
	DarkOverride  = Themed(theme.VariantDark)
)

// darkSurfaces - every color the default dark theme draws as a neutral grey,
// restated as a shade of BrandNavy. The offsets preserve Fyne's own ordering, so
// separators still sit below the background and controls still sit above it.
// Semantic colors (error, warning, success) are deliberately absent so they keep
// their meaning.
var darkSurfaces = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:          BrandNavy,
	theme.ColorNameHeaderBackground:    shade(BrandNavy, 0.02),
	theme.ColorNameOverlayBackground:   shade(BrandNavy, 0.03),
	theme.ColorNameInputBackground:     shade(BrandNavy, 0.05),
	theme.ColorNameScrollBarBackground: shade(BrandNavy, 0.05),
	theme.ColorNameButton:              shade(BrandNavy, 0.09),
	theme.ColorNameDisabledButton:      shade(BrandNavy, 0.09),
	theme.ColorNameMenuBackground:      shade(BrandNavy, 0.09),
	theme.ColorNameInputBorder:         shade(BrandNavy, 0.18),
	theme.ColorNameDisabled:            shade(BrandNavy, 0.28),
	theme.ColorNameSeparator:           shade(BrandNavy, -0.09),
}

// Color on Theme resolves the brand overrides, then defers to the default theme.
func (t *Theme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	// The accent is branding rather than a surface, so it applies to both variants.
	if name == theme.ColorNamePrimary {
		return LogoWaterBlue
	}

	// Surfaces are only retinted in dark mode, so the Light and Dark buttons stay a
	// real choice rather than two shades of blue.
	if t.variant == theme.VariantDark {
		if tinted, ok := darkSurfaces[name]; ok {
			return tinted
		}
	}

	return t.Theme.Color(name, t.variant)
}

// Color resolves a semantic theme colour against the live app theme and its
// current variant, for the raw canvas.Rectangle fills that do not re-theme
// themselves (a caller that rebuilds its container on every navigation
// re-resolves the fill then).
func Color(name fyne.ThemeColorName) color.Color {
	s := fyne.CurrentApp().Settings()
	return s.Theme().Color(name, s.ThemeVariant())
}

// FullBleed - stretch o out to the window edges. The window canvas insets all
// content by theme.Padding() on every side (glCanvas is created padded), so a
// coloured band otherwise stops a few pixels short and leaks the window colour
// down each edge; a matching negative horizontal margin cancels that.
func FullBleed(o fyne.CanvasObject) *fyne.Container {
	pad := theme.Padding()
	return container.New(layout.NewCustomPaddedLayout(0, 0, -pad, -pad), o)
}

// Rule - a full-width horizontal rule h pixels tall in a theme surface colour,
// heavier than widget.NewSeparator()'s 1px hairline.
func Rule(h float32, name fyne.ThemeColorName) fyne.CanvasObject {
	rule := canvas.NewRectangle(Color(name))
	rule.SetMinSize(fyne.NewSize(0, h))

	return rule
}

// AccentBand - put row on a solid LogoWaterBlue band so it reads as a table head
// with real contrast against the window in both themes. The band bleeds to the
// window edges; the row inside is re-inset by theme.Padding() (plus vpad top and
// bottom) so its columns still line up with non-bled content elsewhere. Labels
// in the row are forced to the light palette so they read on the blue.
func AccentBand(row fyne.CanvasObject, vpad float32) fyne.CanvasObject {
	pad := theme.Padding()
	return FullBleed(container.NewStack(
		canvas.NewRectangle(LogoWaterBlue),
		container.New(
			layout.NewCustomPaddedLayout(vpad, vpad, pad, pad),
			container.NewThemeOverride(row, DarkOverride),
		),
	))
}

// CautionStrip - wrap inner in the amber caution fill with the light palette
// forced over it, so its label and buttons read dark on the tint on both app
// themes. Routing every notice strip through here keeps them from drifting apart
// cosmetically. Returned visible; a strip that starts hidden calls Hide() on it.
func CautionStrip(inner fyne.CanvasObject) *fyne.Container {
	return container.NewStack(
		canvas.NewRectangle(BannerAmber),
		container.NewThemeOverride(inner, LightOverride),
	)
}

// ReverseCardColors - the (card, ink) pair for a panel drawn in reverse contrast
// against the window: a BrandNavy card with white text on the light theme, a
// white card with BrandNavy text on the dark theme, so it stands clearly apart
// from the plain surfaces around it. The colours key off the passed variant (the
// in-app theme choice), not Fyne's builtin palette, which tracks the OS
// appearance and can disagree.
func ReverseCardColors(variant fyne.ThemeVariant) (card, ink color.Color) {
	if variant == theme.VariantLight {
		return BrandNavy, White
	}
	return White, BrandNavy
}

// ReverseCard - draw body on a solid reverse-contrast card (ReverseCardColors)
// with the opposite palette forced over it, so the widgets inside read correctly
// on the inverted fill (a navy card wants light text). Pass the app's current
// theme variant. Unlike AccentBand this does not bleed to the window edges; wrap
// the result in FullBleed if that is wanted.
func ReverseCard(variant fyne.ThemeVariant, body fyne.CanvasObject) *fyne.Container {
	card, _ := ReverseCardColors(variant)

	var over fyne.Theme = LightOverride
	if variant == theme.VariantLight {
		over = DarkOverride
	}

	return container.NewStack(
		canvas.NewRectangle(card),
		container.NewThemeOverride(body, over),
	)
}

// shade - return c with its HSL lightness moved by delta, keeping hue and
// saturation intact. Blending toward white instead would desaturate the navy into
// grey, which is what makes a tinted theme look washed out.
func shade(c color.NRGBA, delta float64) color.NRGBA {
	h, s, l := toHSL(c)

	return fromHSL(h, s, math.Min(1, math.Max(0, l+delta)), c.A)
}

func toHSL(c color.NRGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2

	span := max - min
	if span == 0 {
		return 0, 0, l
	}

	if l > 0.5 {
		s = span / (2 - max - min)
	} else {
		s = span / (max + min)
	}

	switch max {
	case r:
		h = math.Mod((g-b)/span+6, 6)
	case g:
		h = (b-r)/span + 2
	default:
		h = (r-g)/span + 4
	}

	return h / 6, s, l
}

func fromHSL(h, s, l float64, alpha uint8) color.NRGBA {
	if s == 0 {
		v := uint8(l*255 + 0.5)
		return color.NRGBA{R: v, G: v, B: v, A: alpha}
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	channel := func(t float64) uint8 {
		t = math.Mod(t+1, 1)
		switch {
		case t < 1.0/6.0:
			return uint8((p+(q-p)*6*t)*255 + 0.5)
		case t < 1.0/2.0:
			return uint8(q*255 + 0.5)
		case t < 2.0/3.0:
			return uint8((p+(q-p)*(2.0/3.0-t)*6)*255 + 0.5)
		default:
			return uint8(p*255 + 0.5)
		}
	}

	return color.NRGBA{R: channel(h + 1.0/3.0), G: channel(h), B: channel(h - 1.0/3.0), A: alpha}
}
