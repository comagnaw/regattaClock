package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2/app"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/regatta"
	"github.com/comagnaw/regattaClock/internal/version"
)

func main() {
	if versionRequested() {
		fmt.Println(version.Get().JSON())
		return
	}

	fyneApp := app.NewWithID(common.AppBundleID)
	defer regatta.Bootstrap(fyneApp)()

	regatta.New(fyneApp).Run()
}

// versionRequested reports whether -v / -version / --version was passed. A bare
// os.Args scan rather than the flag package, which would also claim Fyne's own
// -fyne* flags.
func versionRequested() bool {
	for _, a := range os.Args[1:] {
		switch a {
		case "-v", "-version", "--version":
			return true
		}
	}
	return false
}
