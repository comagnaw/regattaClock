package regatta

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)

type colorTheme struct {
	fyne.Theme

	variant fyne.ThemeVariant
}

var (
	// brandNavy - the deep blue of the branding banner, used as the window
	// background. This is the exact brand value rather than a shade derived from it,
	// so the background matches the artwork instead of merely approximating it.
	brandNavy = color.NRGBA{R: 33, G: 53, B: 76, A: 0xFF}

	// logoWaterBlue - the mid tone covering most of the water in
	// assets/images/RegattaClockBannerSmall.png, used as the accent
	logoWaterBlue = color.NRGBA{R: 0x05, G: 0x69, B: 0xA6, A: 0xFF}
)

// darkSurfaces - every color the default dark theme draws as a neutral grey,
// restated as a shade of brandNavy. The offsets preserve Fyne's own ordering, so
// separators still sit below the background and controls still sit above it.
// Semantic colors (error, warning, success) are deliberately absent so they keep
// their meaning.
var darkSurfaces = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:          brandNavy,
	theme.ColorNameHeaderBackground:    shade(brandNavy, 0.02),
	theme.ColorNameOverlayBackground:   shade(brandNavy, 0.03),
	theme.ColorNameInputBackground:     shade(brandNavy, 0.05),
	theme.ColorNameScrollBarBackground: shade(brandNavy, 0.05),
	theme.ColorNameButton:              shade(brandNavy, 0.09),
	theme.ColorNameDisabledButton:      shade(brandNavy, 0.09),
	theme.ColorNameMenuBackground:      shade(brandNavy, 0.09),
	theme.ColorNameInputBorder:         shade(brandNavy, 0.18),
	theme.ColorNameDisabled:            shade(brandNavy, 0.28),
	theme.ColorNameSeparator:           shade(brandNavy, -0.09),
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

func (f *colorTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	// The accent is branding rather than a surface, so it applies to both variants.
	if name == theme.ColorNamePrimary {
		return logoWaterBlue
	}

	// Surfaces are only retinted in dark mode, so the Light and Dark buttons stay a
	// real choice rather than two shades of blue.
	if f.variant == theme.VariantDark {
		if tinted, ok := darkSurfaces[name]; ok {
			return tinted
		}
	}

	return f.Theme.Color(name, f.variant)
}

func (r *Regatta) setTheme(name string) {
	if name == common.PrefLight {
		r.App.Settings().SetTheme(&colorTheme{Theme: theme.DefaultTheme(), variant: theme.VariantLight})
		r.App.Preferences().SetString(common.PrefTheme, name)
		return
	}
	r.App.Settings().SetTheme(&colorTheme{Theme: theme.DefaultTheme(), variant: theme.VariantDark})
	r.App.Preferences().SetString(common.PrefTheme, common.PrefDark)
}

func (r *Regatta) themeButtons() *fyne.Container {
	return container.NewGridWithColumns(2,
		widget.NewButton(common.PrefDark, func() {
			r.setTheme(common.PrefDark)
		}),
		widget.NewButton(common.PrefLight, func() {
			r.setTheme(common.PrefLight)
		}),
	)
}
