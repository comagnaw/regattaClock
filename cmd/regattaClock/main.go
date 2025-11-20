package main

import (
	"fyne.io/fyne/v2/app"
	"github.com/comagnaw/regattaClock/internal/regatta"
)

func main() {
	fyneApp := app.NewWithID("com.github.comagnaw.regattaClock")
	regattaApp := regatta.NewRegatta(fyneApp)
	regattaApp.Run()
}
