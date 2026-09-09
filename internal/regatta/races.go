package regatta

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"

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
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, 0, 0),
			container.NewCenter(banner(treeWordmarkWidth, treeWordmarkHeight)),
		),
		fullBleed(canvasRule(treeRuleThickness, theme.ColorNameForeground)),
		r.treeTitle(),
	)
	// Every notice banner sits above the column header, so it never pushes into
	// the race list. All are hidden until they apply.
	if r.mode == modeTimer {
		header.Add(r.scheduleBannerWidget())
	} else {
		header.Add(r.directorHeaderExtras())
		r.secondaryLegend = newDismissibleBanner()
		header.Add(r.secondaryLegend.root)
	}
	r.staleLaneLegend = newDismissibleBanner()
	header.Add(r.staleLaneLegend.root)
	header.Add(r.headerBand(r.raceListHeader()))

	// Set the window content
	body := r.raceListBody()
	r.refreshStaleLaneLegend() // rows are realised now; show the legend if any is flagged
	r.refreshSecondaryValueLegend()

	// Keep the reference so the config screen can tell it left the tree and
	// rebuild it (the header's reverse-contrast colours are raw canvas objects
	// that a theme change does not repaint on its own).
	r.treeContent = container.NewBorder(header, nil, nil, nil, body)
	r.window.SetContent(r.treeContent)
}

// treeTitle - the loaded regatta's details as a framed card: a 2x2 grid of
// left-aligned "Key: Value" lines (Regatta / Scheduled Races on the first row,
// Date / Role on the second). The card is painted in reverse contrast against
// the window - a white card with brand-navy text on the dark theme, a brand-navy
// card with white text on the light theme - so it stands clearly apart from the
// wordmark above and the race list below. The colours key off the in-app theme
// choice (r.themeVariant), not Fyne's builtin palette, which tracks the OS
// appearance and can disagree; they are re-applied on every build so a theme
// switch followed by a navigation picks up the change.
func (r *Regatta) treeTitle() *fyne.Container {
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	card, ink := white, brandNavy
	if r.themeVariant == theme.VariantLight {
		card, ink = brandNavy, white
	}
	for _, t := range []*canvas.Text{r.title, r.subtitle, r.date, r.persona} {
		t.Color = ink
	}

	grid := container.NewGridWithColumns(2,
		r.title, r.subtitle,
		r.date, r.persona,
	)
	body := container.New(
		layout.NewCustomPaddedLayout(viewMargin, viewMargin, viewMargin, viewMargin),
		grid,
	)
	panel := container.NewStack(canvas.NewRectangle(card), body)

	return fullBleed(panel)
}

// canvasRule - a full-width horizontal rule h pixels tall in a theme surface
// colour, heavier than widget.NewSeparator()'s 1px hairline.
func canvasRule(h float32, name fyne.ThemeColorName) fyne.CanvasObject {
	rule := canvas.NewRectangle(themeColor(name))
	rule.SetMinSize(fyne.NewSize(0, h))

	return rule
}

// fullBleed - stretch o out to the window edges. The window canvas insets all
// content by theme.Padding() on every side (glCanvas is created padded), so a
// coloured band otherwise stops a few pixels short and leaks the window colour
// down each edge; a matching negative horizontal margin cancels that.
func fullBleed(o fyne.CanvasObject) *fyne.Container {
	pad := theme.Padding()
	return container.New(layout.NewCustomPaddedLayout(0, 0, -pad, -pad), o)
}

// headerBandTheme forces the dark palette for the column-header labels so they
// render light on the blue band regardless of the app's theme choice (a
// ThemeOverride re-themes widgets, which the labels are).
var headerBandTheme fyne.Theme = &colorTheme{Theme: theme.DefaultTheme(), variant: theme.VariantDark}

// headerBand - put the race-list column-header row on a solid accent band so it
// reads as a table head between the details card and the scrolling rows, with
// real contrast against the window in both themes. The band bleeds to the window
// edges; the row inside is re-inset by theme.Padding() so its columns still line
// up exactly with the (non-bled) data rows below.
func (r *Regatta) headerBand(row fyne.CanvasObject) fyne.CanvasObject {
	pad := theme.Padding()
	return fullBleed(container.NewStack(
		canvas.NewRectangle(logoWaterBlue),
		container.New(
			layout.NewCustomPaddedLayout(headerBandVPad, headerBandVPad, pad, pad),
			container.NewThemeOverride(row, headerBandTheme),
		),
	))
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
	default: // RoleDirector - centred to sit over the centred read-only cells.
		cluster = container.NewHBox(
			fixedCell(restartsColWidth, text.BoldLabelCenter(common.ColRestarts)),
			fixedCell(startTimeColWidth, text.BoldLabelCenter(common.ColStartTime)),
			fixedCell(winTimeColWidth, text.BoldLabelCenter(common.ColWinningTime)),
			fixedCell(statusColWidth, text.BoldLabelCenter(common.ColStatus)),
		)
	}

	return container.NewBorder(nil, nil, nil, cluster, race)
}

// fixedCell wraps a widget at a fixed column width so headers and row values
// share one set of column edges. A small horizontal inset keeps a column's
// value off its neighbour to the left (e.g. a right-aligned start time next to
// the Time Race button).
func fixedCell(w float32, o fyne.CanvasObject) *fyne.Container {
	inset := container.New(layout.NewCustomPaddedLayout(0, 0, colInset, colInset), o)
	return container.NewGridWrap(fyne.NewSize(w, inset.MinSize().Height), inset)
}
