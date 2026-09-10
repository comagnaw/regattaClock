package regatta

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/uitheme"
)

func (r *Regatta) setTheme(name string) {
	if name == common.PrefLight {
		r.themeVariant = theme.VariantLight
		r.App.Settings().SetTheme(uitheme.Themed(theme.VariantLight))
		r.App.Preferences().SetString(common.PrefTheme, name)
		return
	}
	r.themeVariant = theme.VariantDark
	r.App.Settings().SetTheme(uitheme.Themed(theme.VariantDark))
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
