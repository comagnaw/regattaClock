// Package assets holds the files compiled into the binary with //go:embed:
// the branding image and the export font. They live together here because an
// embed directive cannot reach outside its own package directory.
package assets

import _ "embed"

// RegattaClockBannerSmall - branding banner shown on the welcome view and the
// race-tree header. The full-resolution source artwork and the horizontal
// wordmark draft sit in images/ but are not embedded.
//
//go:embed images/RegattaClockBannerSmall.png
var RegattaClockBannerSmall []byte
