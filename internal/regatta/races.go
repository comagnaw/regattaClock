package regatta

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/clock"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func (r *Regatta) showRaceTree() {
	if r.RegattaData == nil {
		return
	}

	// Create a container for the race tree
	mainContainer := container.NewVBox(
		r.treeTitle(),
		widget.NewSeparator(),
		r.listTitle(),
		r.raceList(),
	)

	// Set the window content
	r.window.SetContent(mainContainer)
	r.window.Resize(fyne.NewSize(500, 600))
}

func (r *Regatta) treeTitle() *fyne.Container {
	// Add regatta information at the top
	return container.NewVBox(
		container.NewCenter(r.title),
		container.NewCenter(r.subtitle),
		container.NewCenter(r.date),
	)
}

func (r *Regatta) listTitle() *widget.Label {
	// Add a title for the race list
	title := widget.NewLabel("Scheduled Races")
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

	// Create a scroll container for the race list
	scroll := container.NewScroll(raceList)
	scroll.SetMinSize(fyne.NewSize(500, 600))
	return scroll
}

func (r *Regatta) timeButton(race reader.RaceData) *widget.Button {
	// Create a button to time this race
	return widget.NewButton("Time Race", func(raceData reader.RaceData) func() {
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
