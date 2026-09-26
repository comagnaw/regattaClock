package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2/app"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/regatta"
	"github.com/comagnaw/regattaClock/internal/sample"
	"github.com/comagnaw/regattaClock/internal/version"
)

func main() {
	if versionRequested() {
		fmt.Println(version.Get().JSON())
		return
	}
	if opts, ok, err := sampleRequested(os.Args[1:]); err != nil || ok {
		os.Exit(writeSample(opts, err))
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

// writeSample generates the sample race day for -dev-sample-regatta and
// returns the process exit code. It runs before app.NewWithID, so no window
// opens.
func writeSample(opts sample.Options, parseErr error) int {
	if parseErr != nil {
		fmt.Fprintln(os.Stderr, "regattaClock:", parseErr)
		return 2
	}
	rep, err := sample.Generate(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "regattaClock: sample regatta:", err)
		return 1
	}
	fmt.Print(rep)
	return 0
}

// Hidden developer flags that write a sample race day and exit
// (docs/features/testing/sample-regatta.md). Not in any help text.
const (
	flagSampleRegatta = "dev-sample-regatta" // <dir> - the artifact root
	flagSampleUnraced = "dev-sample-unraced" // <n>, default sample.DefaultUnraced
	flagSampleDate    = "dev-sample-date"    // <YYYY-MM-DD>, default today
)

// sampleRequested scans args (os.Args[1:]) for the -dev-sample-* flags. A
// bare scan rather than the flag package, which would also claim Fyne's own
// -fyne* flags - the same reason as versionRequested. Each flag takes its
// value as the next argument or after "=", with one or two leading dashes.
// ok is false when -dev-sample-regatta is absent; the other two flags alone
// are an error.
func sampleRequested(args []string) (opts sample.Options, ok bool, err error) {
	opts.Unraced = sample.DefaultUnraced
	var sawOther bool
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if !strings.HasPrefix(args[i], "-") {
			continue
		}
		switch name {
		case flagSampleRegatta, flagSampleUnraced, flagSampleDate:
		default:
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return sample.Options{}, false, fmt.Errorf("-%s needs a value", name)
			}
			i++
			value = args[i]
		}
		switch name {
		case flagSampleRegatta:
			if value == "" {
				return sample.Options{}, false, fmt.Errorf("-%s needs a directory", name)
			}
			opts.Dir, ok = value, true
		case flagSampleUnraced:
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return sample.Options{}, false, fmt.Errorf("-%s %q: want a whole number, 0 or more", name, value)
			}
			opts.Unraced, sawOther = n, true
		case flagSampleDate:
			d, err := time.ParseInLocation("2006-01-02", value, time.Local)
			if err != nil {
				return sample.Options{}, false, fmt.Errorf("-%s %q: want YYYY-MM-DD", name, value)
			}
			opts.Date, sawOther = d, true
		}
	}
	if sawOther && !ok {
		return sample.Options{}, false, errors.New("-" + flagSampleUnraced + " and -" + flagSampleDate + " need -" + flagSampleRegatta + " <dir>")
	}
	return opts, ok, nil
}
