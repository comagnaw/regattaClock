package regatta

import (
	"image/color"

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

func (f *colorTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
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
