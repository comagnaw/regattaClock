package regatta

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/text"
	"github.com/comagnaw/regattaClock/internal/uitheme"
)

func (r *Regatta) showRaceTree() {
	if r.RegattaData == nil {
		return
	}

	// Border rather than VBox so the race list takes whatever height is left over
	// after the header. In a VBox the list would report its own minimum on top of
	// the header's, forcing the window taller than regattaHeight.
	header := container.NewVBox(
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, 0, 0),
			container.NewCenter(banner(treeWordmarkWidth, treeWordmarkHeight)),
		),
		uitheme.FullBleed(uitheme.Rule(treeRuleThickness, theme.ColorNameForeground)),
		r.treeTitle(),
	)
	// Every notice banner sits above the column header, so it never pushes into
	// the race list. All are hidden until they apply.
	if r.mode == modeTimer {
		header.Add(r.scheduleBannerWidget())
	} else {
		header.Add(r.directorHeaderExtras())
	}
	r.staleLaneLegend = newDismissibleBanner()
	header.Add(r.staleLaneLegend.root)
	header.Add(uitheme.AccentBand(r.raceListHeader(), headerBandVPad))

	// Set the window content
	body := r.raceListBody()
	r.refreshStaleLaneLegend() // rows are realised now; show the legend if any is flagged

	// Keep the reference so the config screen can tell it left the tree and
	// rebuild it (the header's reverse-contrast colours are raw canvas objects
	// that a theme change does not repaint on its own).
	r.treeContent = container.NewBorder(header, nil, nil, nil, body)
	r.window.SetContent(r.treeContent)
}

// treeTitle - the loaded regatta's details as a framed card: two independently
// sized Key:/Value column blocks side by side - the left holding Regatta and
// Scheduled Races, the right holding Date and Role - so each column's own
// colons line up regardless of how long its two keys are. A single
// leading-aligned "Key: Value" string per cell left the colons staggered; a
// single uniform 8-column grid instead sized every column to the single
// widest cell, ballooning the card's width, so each side is its own
// layout.FormLayout (label column sized to its own two keys, value column
// taking the rest). The card is painted in reverse contrast against the
// window - a white card with brand-navy text on the dark theme, a brand-navy
// card with white text on the light theme - so it stands clearly apart from
// the wordmark above and the race list below. The colours key off the in-app
// theme choice (r.themeVariant), not Fyne's builtin palette, which tracks the
// OS appearance and can disagree; they are re-applied on every build so a
// theme switch followed by a navigation picks up the change.
func (r *Regatta) treeTitle() *fyne.Container {
	card, ink := uitheme.ReverseCardColors(r.themeVariant)
	for _, t := range []*canvas.Text{r.title, r.subtitle, r.date, r.persona} {
		t.Color = ink
	}

	key := func(label string) *canvas.Text {
		t := text.Header3(label)
		t.Alignment = fyne.TextAlignTrailing
		t.Color = ink
		return t
	}

	left := container.New(layout.NewFormLayout(),
		key(common.TreeRegattaKey), r.title,
		key(common.TreeScheduledRacesKey), r.subtitle,
	)
	right := container.New(layout.NewFormLayout(),
		key(common.TreeDateKey), r.date,
		key(common.TreeRoleKey), r.persona,
	)
	grid := container.NewHBox(left, right)
	body := container.New(
		layout.NewCustomPaddedLayout(viewMargin, viewMargin, viewMargin, viewMargin),
		grid,
	)
	panel := container.NewStack(canvas.NewRectangle(card), body)

	return uitheme.FullBleed(panel)
}

// raceListHeader is the bold column-header row above the race list. It uses
// the same Border(nil,nil,left,cluster,title) shape and the same fixed
// column widths as a data row (newRaceRow) - Race number and Scheduled Time
// left-anchored ahead of the race title, the rest right-anchored - so each
// label sits directly over its column: the same shared column set for every
// role, plus a role-specific, unlabelled action cell where a data row has
// its buttons.
func (r *Regatta) raceListHeader() *fyne.Container {
	race := text.BoldLabel(common.ScheduledRacesTile)
	race.Alignment = fyne.TextAlignTrailing

	var action *fyne.Container
	switch r.session.Role {
	case persona.RoleStart:
		action = fixedCell(actionsColWidth, text.BoldLabel(common.EmptyString))
	case persona.RoleFinish:
		action = fixedCell(timeRaceColWidth, text.BoldLabel(common.EmptyString))
	}

	var cells []fyne.CanvasObject
	if action != nil {
		cells = append(cells, action)
	}
	cells = append(cells,
		fixedCell(restartsColWidth, text.BoldLabelCenter(common.ColRestarts)),
		fixedCell(startTimeColWidth, text.BoldLabelCenter(common.ColStartTime)),
		fixedCell(winTimeColWidth, text.BoldLabelCenter(common.ColWinningTime)),
		fixedCell(statusColWidth, text.BoldLabelCenter(common.ColStatus)),
	)

	leading := container.NewHBox(
		fixedCell(raceNumColWidth, text.BoldLabelCenter(common.ColRace)),
		fixedCell(scheduledTimeColWidth, text.BoldLabelCenter(common.ColScheduledTime)),
	)
	return container.NewBorder(nil, nil, leading, container.NewHBox(cells...), race)
}

// fixedCell wraps a widget at a fixed column width so headers and row values
// share one set of column edges. A small horizontal inset keeps a column's
// value off its neighbour to the left (e.g. a right-aligned start time next to
// the Time Race button).
func fixedCell(w float32, o fyne.CanvasObject) *fyne.Container {
	inset := container.New(layout.NewCustomPaddedLayout(0, 0, colInset, colInset), o)
	return container.NewGridWrap(fyne.NewSize(w, inset.MinSize().Height), inset)
}
