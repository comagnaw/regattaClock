package main

import (
	"fyne.io/fyne/v2/app"
	"github.com/comagnaw/regattaClock"
)

func main() {
	fyneApp := app.NewWithID("com.github.comagnaw.regattaClock")
	regattaApp := regattaClock.NewApp(fyneApp)
	regattaApp.Run()
}
