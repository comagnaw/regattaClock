package regatta

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/text"
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
		r.raceListHeader(),
	)
	if r.mode == modeTimer {
		header.Add(r.scheduleBannerWidget())
	} else {
		header.Add(r.directorHeaderExtras())
	}
	header.Add(r.staleLaneLegendWidget())

	// Set the window content
	body := r.raceListBody()
	r.refreshStaleLaneLegend() // rows are realised now; show the legend if any is flagged
	r.window.SetContent(container.NewBorder(header, nil, nil, nil, body))
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

// raceListHeader is the bold column-header row above the race list. It uses the
// same Border(nil,nil,nil,cluster,title) shape and the same fixed column widths
// as a data row, so each label sits directly over its column and "Scheduled
// Races" right-aligns to line up with the race titles below it.
func (r *Regatta) raceListHeader() *fyne.Container {
	race := text.BoldLabel(common.ScheduledRacesTile)
	race.Alignment = fyne.TextAlignTrailing

	var cluster *fyne.Container
	switch r.session.Role {
	case persona.RoleStart:
		cluster = container.NewHBox(
			fixedCell(actionsColWidth, text.BoldLabel(common.EmptyString)),
			fixedCell(startTimeColWidth, text.BoldLabel(common.ColStartTime)),
			fixedCell(statusColWidth, text.BoldLabel(common.ColStatus)),
		)
	case persona.RoleFinish:
		cluster = container.NewHBox(
			fixedCell(timeRaceColWidth, text.BoldLabel(common.EmptyString)),
			fixedCell(startTimeColWidth, text.BoldLabel(common.ColStartTime)),
			fixedCell(statusColWidth, text.BoldLabel(common.ColStatus)),
		)
	default: // RoleDirector
		cluster = container.NewHBox(
			fixedCell(restartsColWidth, text.BoldLabel(common.ColRestarts)),
			fixedCell(startTimeColWidth, text.BoldLabel(common.ColStartTime)),
			fixedCell(winTimeColWidth, text.BoldLabel(common.ColWinningTime)),
			fixedCell(statusColWidth, text.BoldLabel(common.ColStatus)),
		)
	}

	return container.NewBorder(nil, nil, nil, cluster, race)
}

// fixedCell wraps a widget at a fixed column width so headers and row values
// share one set of column edges.
func fixedCell(w float32, o fyne.CanvasObject) *fyne.Container {
	return container.NewGridWrap(fyne.NewSize(w, o.MinSize().Height), o)
}
