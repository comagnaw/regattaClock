// Package assets holds the image files compiled into the binary. The embed
// directive cannot reach outside its own directory, so this declaration lives
// alongside the images rather than in internal/assets with the fonts.
package assets

import _ "embed"

// RegattaClockBannerSmall - branding banner shown on the welcome view
//
//go:embed images/RegattaClockBannerSmall.png
var RegattaClockBannerSmall []byte
