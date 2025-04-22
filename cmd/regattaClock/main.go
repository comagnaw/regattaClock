package main

import (
	"fyne.io/fyne/v2/app"
	"github.com/comagnaw/regattaClock"
)

func main() {
	fyneApp := app.New()
	regattaApp := regattaClock.NewApp(fyneApp)
	regattaApp.Run()
}
