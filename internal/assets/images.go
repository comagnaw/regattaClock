// Package assets holds the files compiled into the binary with //go:embed:
// the branding wordmark and the export font. They live together here because an
// embed directive cannot reach outside its own package directory.
package assets

import _ "embed"

// RegattaClockBanner - the branding wordmark (SVG), shown on the welcome-family
// views and the race-tree header. It is a single-fill path, so it is wrapped in
// a themed resource at use time and takes the current theme's foreground color.
// The source artwork (RegattaClockBanner.png) and the light-mode README variant
// (RegattaClockBanner-onlight.svg) sit in images/ but are not embedded.
//
//go:embed images/RegattaClockBanner.svg
var RegattaClockBanner []byte
