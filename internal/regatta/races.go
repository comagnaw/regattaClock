package regatta

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/clock"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func (r *Regatta) showRaceTree() {
	if r.RegattaData == nil {
		return
	}

	// Border rather than VBox so the race list takes whatever height is left over
	// after the header. In a VBox the list would report its own minimum on top of
	// the header's, forcing the window taller than regattaHeight.
	header := container.NewVBox(
		r.treeTitle(),
		widget.NewSeparator(),
		r.listTitle(),
	)

	// Set the window content
	r.window.SetContent(container.NewBorder(header, nil, nil, nil, r.raceList()))
}

// treeTitle - loaded regatta details, with the branding logo tucked into the top
// left corner beside them. The operator's role sits above the regatta name when
// a session is bound.
func (r *Regatta) treeTitle() *fyne.Container {
	lines := make([]fyne.CanvasObject, 0, 4)
	if r.persona != nil && r.persona.Text != common.EmptyString {
		lines = append(lines, container.NewCenter(r.persona))
	}
	lines = append(lines,
		container.NewCenter(r.title),
		container.NewCenter(r.subtitle),
		container.NewCenter(r.date),
	)
	details := container.NewVBox(lines...)

	// Border hands the left slot the full height of the details block, and
	// ImageFillContain keeps the logo at its aspect ratio centred within it.
	logo := container.New(
		layout.NewCustomPaddedLayout(0, 0, viewMargin, 0),
		banner(treeBannerWidth, treeBannerHeight),
	)

	return container.NewBorder(nil, nil, logo, nil, details)
}

func (r *Regatta) listTitle() *widget.Label {
	// Add a title for the race list
	title := widget.NewLabel(common.ScheduledRacesTile)
	title.TextStyle = fyne.TextStyle{Bold: true}
	return title
}

func (r *Regatta) raceList() *container.Scroll {

	// Create a list to hold the race nodes
	raceList := container.NewVBox()

	// Add each race to the tree
	for _, race := range r.RegattaData.SortedRaces() {

		if !race.HasBoats() {
			continue
		}

		raceList.Add(r.raceEntry(race))
	}

	// Only a floor to keep a few rows visible if the window is dragged small. Giving
	// this the full window height instead would make the content taller than the
	// window, since the header sits above it.
	scroll := container.NewScroll(raceList)
	scroll.SetMinSize(fyne.NewSize(0, raceListMinHeight))
	return scroll
}

func (r *Regatta) timeButton(race reader.RaceData) *widget.Button {
	// Create a button to time this race
	return widget.NewButton(common.TimeRaceButtonText, func(raceData reader.RaceData) func() {
		return func() {
			clockApp := clock.NewClock(r.App, r.RegattaData, raceData)
			clockApp.OpenRaceClock()

		}
	}(race))
}

func (r *Regatta) raceEntry(race reader.RaceData) *fyne.Container {
	return container.NewHBox(
		widget.NewLabel(race.RaceTitle()),
		layout.NewSpacer(),
		r.timeButton(race),
	)
}
