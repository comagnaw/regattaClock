package main

import (
	"fyne.io/fyne/v2/app"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/regatta"
)

func main() {
	fyneApp := app.NewWithID(common.AppBundleID)
	defer regatta.Bootstrap(fyneApp)()

	regatta.NewDirector(fyneApp).Run()
}
