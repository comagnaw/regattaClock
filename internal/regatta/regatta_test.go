package regatta

import (
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// labelTexts collects the text of every widget.Label anywhere under o.
func labelTexts(o fyne.CanvasObject) []string {
	var out []string
	switch v := o.(type) {
	case *widget.Label:
		out = append(out, v.Text)
	case *fyne.Container:
		for _, c := range v.Objects {
			out = append(out, labelTexts(c)...)
		}
	}
	return out
}

// leafTexts collects label and button text under o in visual (tree) order, so a
// test can assert one column sits ahead of another.
func leafTexts(o fyne.CanvasObject) []string {
	switch v := o.(type) {
	case *widget.Button:
		return []string{v.Text}
	case *widget.Label:
		return []string{v.Text}
	case *fyne.Container:
		var out []string
		for _, c := range v.Objects {
			out = append(out, leafTexts(c)...)
		}
		return out
	}
	return nil
}

// ftRegatta is a NewTimer bound to a Primary Finish Timer session, enough for
// the pure layout helpers (raceListHeader, newRaceRow).
func ftRegatta(t *testing.T) *Regatta {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)

	r := NewTimer(app)
	def, ok := persona.ByID("pft")
	if !ok {
		t.Fatal("no pft persona")
	}
	r.session = persona.Session{Definition: def}
	return r
}

func TestRegatta_RaceListHeader_Finish(t *testing.T) {
	texts := labelTexts(ftRegatta(t).raceListHeader())

	for _, want := range []string{common.ScheduledRacesTile, common.ColStartTime, common.ColStatus} {
		if !slices.Contains(texts, want) {
			t.Errorf("finish header %v is missing %q", texts, want)
		}
	}
	if i, j := slices.Index(texts, common.ColStartTime), slices.Index(texts, common.ColStatus); i < 0 || j < 0 || i > j {
		t.Errorf("finish header %v: want %q before %q", texts, common.ColStartTime, common.ColStatus)
	}
}

func TestRegatta_FinishRow_TimeRaceLeadsCluster(t *testing.T) {
	r := ftRegatta(t)
	row := r.newRaceRow(reader.RaceData{
		RaceNumber: 1, BoatCount: 2,
		Lanes: map[int]reader.RaceEntry{1: {SchoolName: "A"}, 2: {SchoolName: "B"}},
	})

	seq := leafTexts(row.root)
	btn := slices.Index(seq, common.TimeRaceButtonText)
	start := slices.Index(seq, common.WaitingForStartText)
	if btn < 0 || start < 0 || btn > start {
		t.Errorf("finish row leaf order %v: want %q before the start-time cell %q",
			seq, common.TimeRaceButtonText, common.WaitingForStartText)
	}

	// The start-time cell must clip rather than bleed onto the Time Race button
	// now sitting directly to its left.
	if row.startTime.Truncation != fyne.TextTruncateEllipsis {
		t.Errorf("start-time label truncation = %v, want ellipsis", row.startTime.Truncation)
	}
}

func TestNewDirector(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	if regatta == nil {
		t.Fatal("NewRegatta returned nil")
	}

	if regatta.App != app {
		t.Error("App reference not set correctly")
	}

	if regatta.window == nil {
		t.Error("Window should be initialized")
	}

	if regatta.title == nil {
		t.Error("Title should be initialized")
	}

	if regatta.subtitle == nil {
		t.Error("Subtitle should be initialized")
	}

	if regatta.date == nil {
		t.Error("Date should be initialized")
	}

	// Verify initial text is empty
	if regatta.title.Text != common.EmptyString {
		t.Errorf("Expected empty title, got %q", regatta.title.Text)
	}

	if regatta.subtitle.Text != common.EmptyString {
		t.Errorf("Expected empty subtitle, got %q", regatta.subtitle.Text)
	}

	if regatta.date.Text != common.EmptyString {
		t.Errorf("Expected empty date, got %q", regatta.date.Text)
	}

	// NewRegatta initialises RegattaData, so a fresh app holds an empty regatta
	// rather than a nil pointer.
	if regatta.RegattaData == nil {
		t.Fatal("RegattaData should be initialized")
	}

	if len(regatta.RegattaData.Races) != 0 {
		t.Errorf("Expected no races initially, got %d", len(regatta.RegattaData.Races))
	}

	if regatta.RegattaData.Name != common.EmptyString {
		t.Errorf("Expected empty regatta name, got %q", regatta.RegattaData.Name)
	}
}

func TestRegatta_RefreshContent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	// Create mock regatta data
	regatta.RegattaData = &reader.RegattaData{
		Name: "Test Regatta",
		Date: "2024-01-15",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  4,
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School A"}},
			},
			{
				RaceNumber: 2,
				BoatCount:  3,
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School B"}},
			},
		},
	}

	// Ensure window has content before calling refresh
	regatta.showRaceTree()
	regatta.refreshContent()

	expectedTitle := "Test Regatta"
	if regatta.title.Text != expectedTitle {
		t.Errorf("Expected title %q, got %q", expectedTitle, regatta.title.Text)
	}

	expectedDate := "2024-01-15"
	if regatta.date.Text != expectedDate {
		t.Errorf("Expected date %q, got %q", expectedDate, regatta.date.Text)
	}

	expectedSubtitle := "2"
	if regatta.subtitle.Text != expectedSubtitle {
		t.Errorf("Expected subtitle %q, got %q", expectedSubtitle, regatta.subtitle.Text)
	}
}

func TestRegatta_RefreshContent_NoRaces(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	regatta.RegattaData = &reader.RegattaData{
		Name:  "Empty Regatta",
		Date:  "2024-02-20",
		Races: []reader.RaceData{},
	}

	regatta.showRaceTree()
	regatta.refreshContent()

	if regatta.title.Text != "Empty Regatta" {
		t.Errorf("Expected title 'Empty Regatta', got %q", regatta.title.Text)
	}

	expectedSubtitle := "0"
	if regatta.subtitle.Text != expectedSubtitle {
		t.Errorf("Expected subtitle %q, got %q", expectedSubtitle, regatta.subtitle.Text)
	}
}

func TestRegatta_RefreshContent_SomeEmptyRaces(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	regatta.RegattaData = &reader.RegattaData{
		Name: "Partial Regatta",
		Date: "2024-03-10",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  4,
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School A"}},
			},
			{
				RaceNumber: 2,
				BoatCount:  0,
				Lanes:      map[int]reader.RaceEntry{}, // No boats
			},
			{
				RaceNumber: 3,
				BoatCount:  3,
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School C"}},
			},
		},
	}

	regatta.showRaceTree()
	regatta.refreshContent()

	// Should only count races with boats
	expectedSubtitle := "2"
	if regatta.subtitle.Text != expectedSubtitle {
		t.Errorf("Expected subtitle %q, got %q", expectedSubtitle, regatta.subtitle.Text)
	}
}

func TestRegatta_ShowRaceTree_NilRegattaData(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)
	regatta.RegattaData = nil

	// Should not panic with nil RegattaData
	regatta.showRaceTree()

	// Window content should not be changed
	if regatta.window.Content() != nil {
		// Content may exist from initialization, just verify no panic occurred
		t.Log("showRaceTree with nil RegattaData completed without panic")
	}
}

func TestRegatta_ShowRaceTree_WithData(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	regatta.RegattaData = &reader.RegattaData{
		Name: "Test Regatta",
		Date: "2024-01-15",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  4,
				BoatClass:  "Varsity 8",
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School A"}},
			},
			{
				RaceNumber: 2,
				BoatCount:  3,
				BoatClass:  "JV 4",
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School B"}},
			},
		},
	}

	regatta.showRaceTree()

	if regatta.window.Content() == nil {
		t.Error("Window content should be set after showRaceTree")
	}

	// Verify window size
	size := regatta.window.Canvas().Size()
	if size.Width != regattaWidth || size.Height != regattaHeight {
		t.Errorf("Expected window size %fx%f, got %fx%f",
			regattaWidth, regattaHeight, size.Width, size.Height)
	}
}

func TestRegatta_TreeTitle(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)
	regatta.title.Text = "Test Title"
	regatta.subtitle.Text = "Test Subtitle"
	regatta.date.Text = "Test Date"

	titleRow := regatta.treeTitle()

	if titleRow == nil {
		t.Fatal("treeTitle returned nil")
	}

	// Assert on what the row holds rather than its nesting, so wrapping it in a
	// different layout does not break the test. The wordmark is a sibling row in
	// showRaceTree now, not part of treeTitle.
	images, texts := countObjects(titleRow)

	if images != 0 {
		t.Errorf("treeTitle should carry no image, got %d", images)
	}

	// regatta / scheduled races / date / role - four Key:/Value pairs, always
	// present (the role value is an empty *canvas.Text when no session is
	// bound; its "Role:" key still renders).
	if texts != 8 {
		t.Errorf("Expected the four Key: Value pairs (8 text objects), got %d", texts)
	}
}

// keyTexts collects every right-aligned "Key:" *canvas.Text under o, in tree
// order - the four labels treeTitle builds, grouped two-by-two per column.
func keyTexts(o fyne.CanvasObject) []*canvas.Text {
	var out []*canvas.Text
	switch v := o.(type) {
	case *canvas.Text:
		if v.Alignment == fyne.TextAlignTrailing {
			out = append(out, v)
		}
	case *fyne.Container:
		for _, c := range v.Objects {
			out = append(out, keyTexts(c)...)
		}
	}
	return out
}

// TestRegatta_TreeTitleKeysAlignPerColumn - docs/features/PRE-RELEASE-BUGS.md
// Feature 6: a single 8-column grid sized every column to the single widest
// cell (ballooning the card), so treeTitle instead uses one layout.FormLayout
// per column (Regatta/Date on the left, Scheduled Races/Role on the right).
// Each FormLayout sizes its own label column to the wider of its own two
// keys, so within a column both keys end up the same width - the colons line
// up - without forcing the *other* column's keys to match too.
func TestRegatta_TreeTitleKeysAlignPerColumn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)
	panel := regatta.treeTitle()
	panel.Resize(panel.MinSize())

	keys := keyTexts(panel)
	if len(keys) != 4 {
		t.Fatalf("expected 4 key labels, got %d", len(keys))
	}

	// keys[0..1] = left column (Regatta:, Scheduled Races:); keys[2..3] = right
	// column (Date:, Role:) - see treeTitle's left/right construction order.
	if w0, w1 := keys[0].Size().Width, keys[1].Size().Width; w0 != w1 {
		t.Errorf("left column keys %q/%q have different widths (%v/%v) - colons will not align",
			keys[0].Text, keys[1].Text, w0, w1)
	}
	if w2, w3 := keys[2].Size().Width, keys[3].Size().Width; w2 != w3 {
		t.Errorf("right column keys %q/%q have different widths (%v/%v) - colons will not align",
			keys[2].Text, keys[3].Text, w2, w3)
	}
}

func TestRegatta_TreeHeaderHasWordmark(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))

	r := NewDirector(app)
	stopWatch(t, r)

	if onWelcome(r) {
		t.Fatal("precondition: NewDirector should restore into the race tree")
	}
	if images, _ := countObjects(r.window.Content()); images < 1 {
		t.Error("the race-tree header should show the branding wordmark")
	}
}

func TestRegatta_TreeTitleWithRole(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)
	regatta.persona.Text = "Whatever"

	if _, texts := countObjects(regatta.treeTitle()); texts != 8 {
		t.Errorf("Expected the role pair plus title, subtitle and date pairs (8 text objects), got %d", texts)
	}
}

func TestDirector_ShowsRoleAfterRestore(t *testing.T) {
	app := test.NewTempApp(t)
	root := seedRegatta(t, testSchedule())
	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))

	r := NewDirector(app)

	if r.persona.Text != "Regatta Director" {
		t.Errorf("header role line = %q, want %q", r.persona.Text, "Regatta Director")
	}
	if got := r.window.Title(); got != "Regatta Clock — Regatta Director" {
		t.Errorf("window title = %q, want %q", got, "Regatta Clock — Regatta Director")
	}
}

// countObjects - tally the images and text objects anywhere under o
func countObjects(o fyne.CanvasObject) (images, texts int) {
	switch obj := o.(type) {
	case *canvas.Image:
		images++
	case *canvas.Text:
		texts++
	case *fyne.Container:
		for _, child := range obj.Objects {
			childImages, childTexts := countObjects(child)
			images += childImages
			texts += childTexts
		}
	}
	return images, texts
}

func TestRegatta_RaceListHeader_Director(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)
	header := regatta.raceListHeader()
	if header == nil {
		t.Fatal("raceListHeader returned nil")
	}

	texts := labelTexts(header)
	for _, want := range []string{
		common.ScheduledRacesTile, common.ColRestarts, common.ColStartTime,
		common.ColWinningTime, common.ColStatus,
	} {
		if !slices.Contains(texts, want) {
			t.Errorf("director header %v is missing %q", texts, want)
		}
	}
}

func directorWithRaces(t *testing.T, races []reader.RaceData) *Regatta {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)

	r := NewDirector(app)
	r.session = persona.Session{Definition: persona.DirectorDefinition}
	r.RegattaData = &reader.RegattaData{Name: "Test", Date: "2026-10-02", Races: races}
	return r
}

func TestRegatta_DirectorRaceList(t *testing.T) {
	r := directorWithRaces(t, []reader.RaceData{
		{RaceNumber: 1, BoatCount: 4, Lanes: map[int]reader.RaceEntry{1: {SchoolName: "A"}}},
		{RaceNumber: 2, BoatCount: 0, Lanes: map[int]reader.RaceEntry{}}, // skipped
		{RaceNumber: 3, BoatCount: 3, Lanes: map[int]reader.RaceEntry{1: {SchoolName: "C"}}},
	})

	scroll := r.raceListBody()
	if scroll == nil {
		t.Fatal("directorRaceList returned nil")
	}
	if h := scroll.MinSize().Height; h < raceListMinHeight || h >= regattaHeight {
		t.Errorf("scroll min height %f outside [%f, %f)", h, raceListMinHeight, regattaHeight)
	}

	if len(r.rows) != 2 {
		t.Fatalf("rows = %d, want 2 (race 2 has no boats)", len(r.rows))
	}
	if _, skipped := r.rows[2]; skipped {
		t.Error("a race with no boats should not get a row")
	}
}

func TestRegatta_DirectorRow_LayoutAndPlaceholders(t *testing.T) {
	r := directorWithRaces(t, []reader.RaceData{
		{RaceNumber: 5, BoatCount: 4, BoatClass: "Varsity 8", FlightInfo: "Heat 1",
			Lanes: map[int]reader.RaceEntry{1: {SchoolName: "School A"}}},
	})
	r.raceListBody()

	row := r.rows[5]
	if row == nil {
		t.Fatal("no row for race 5")
	}
	if row.title.Alignment != fyne.TextAlignTrailing {
		t.Error("the race title should be right-justified")
	}
	if row.startBtn != nil || row.timeBtn != nil {
		t.Error("a director row has no buttons")
	}
	// No timing logs bound yet: every metric cell shows its placeholder.
	wantNotStarted := store.StateNotStarted.DisplayText(persona.TeamPrimary)
	if row.restarts.Text != common.NoStartTimeText ||
		row.startTime.Text != common.NoStartTimeText ||
		row.winTime.Text != common.NoStartTimeText ||
		row.approved.Text != wantNotStarted {
		t.Errorf("placeholder cells wrong: restarts=%q start=%q win=%q status=%q",
			row.restarts.Text, row.startTime.Text, row.winTime.Text, row.approved.Text)
	}
}

func TestRegatta_MakeMenu(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	menu := regatta.makeMenu()

	if menu == nil {
		t.Fatal("makeMenu returned nil")
	}

	if len(menu.Items) != 1 {
		t.Errorf("Expected 1 main menu, got %d", len(menu.Items))
	}

	mainMenu := menu.Items[0]
	if mainMenu.Label != common.AppTitle {
		t.Errorf("Expected menu label %q, got %q", common.AppTitle, mainMenu.Label)
	}

	// Should have at least 3 menu items plus separator
	if len(mainMenu.Items) < 4 {
		t.Errorf("Expected at least 4 menu items, got %d", len(mainMenu.Items))
	}
}

func TestRegatta_ImportItem(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	item := regatta.importItem()

	if item == nil {
		t.Fatal("importItem returned nil")
	}

	if item.Label != common.LoadDataTitle {
		t.Errorf("Expected label %q, got %q", common.LoadDataTitle, item.Label)
	}

	if item.Action == nil {
		t.Error("Import item should have an action")
	}
}

func TestRegatta_ShowWindowItem(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	item := regatta.showWindowItem()

	if item == nil {
		t.Fatal("showWindowItem returned nil")
	}

	if item.Label != common.ShowWindowText {
		t.Errorf("Expected label %q, got %q", common.ShowWindowText, item.Label)
	}

	if item.Action == nil {
		t.Error("Show window item should have an action")
	}
}

func TestRegatta_ExitItem(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	item := regatta.exitItem()

	if item == nil {
		t.Fatal("exitItem returned nil")
	}

	if item.Label != common.ExitButtonText {
		t.Errorf("Expected label %q, got %q", common.ExitButtonText, item.Label)
	}

	if item.Action == nil {
		t.Error("Exit item should have an action")
	}
}

func TestRegatta_InitialWindowSize(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)

	size := regatta.window.Canvas().Size()
	if size.Width != regattaWidth || size.Height != regattaHeight {
		t.Errorf("Expected initial window size %fx%f, got %fx%f",
			regattaWidth, regattaHeight, size.Width, size.Height)
	}
}
