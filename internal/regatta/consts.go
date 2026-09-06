package regatta

const (
	regattaWidth  = float32(800)
	regattaHeight = float32(600)

	// viewMargin - inset keeping view content off the window edge
	viewMargin = float32(20)

	// raceListMinHeight - floor for the scrolling race list. Deliberately well under
	// regattaHeight: the list shares the window with the title header, so a taller
	// floor would push the content past the window and force it to grow.
	raceListMinHeight = float32(120)

	// welcomeBannerWidth, welcomeBannerHeight - banner size on the welcome view,
	// keeping the source image's 16:9 ratio
	welcomeBannerWidth  = float32(320)
	welcomeBannerHeight = float32(180)

	// treeBannerWidth, treeBannerHeight - banner size beside the regatta title on
	// the race list. Kept under the three line title block's height so the logo
	// never drives the row taller and shifts the list down.
	treeBannerWidth  = float32(150)
	treeBannerHeight = float32(84)
)
