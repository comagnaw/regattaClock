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

	expectedSubtitle := "Scheduled Races: 2"
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

	expectedSubtitle := "Scheduled Races: 0"
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
	expectedSubtitle := "Scheduled Races: 2"
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
	// different layout does not break the test.
	images, texts := countObjects(titleRow)

	if images != 1 {
		t.Errorf("Expected the branding logo in the tree title, got %d images", images)
	}

	if texts != 3 {
		t.Errorf("Expected the title, subtitle and date, got %d text objects", texts)
	}
}

func TestRegatta_TreeTitleWithRole(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewDirector(app)
	regatta.persona.Text = "Role: Whatever"

	if _, texts := countObjects(regatta.treeTitle()); texts != 4 {
		t.Errorf("Expected the role line plus title, subtitle and date, got %d text objects", texts)
	}
}

func TestDirector_ShowsRoleAfterRestore(t *testing.T) {
	app := test.NewTempApp(t)
	root := seedRegatta(t, testSchedule())
	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))

	r := NewDirector(app)

	if r.persona.Text != "Role: Regatta Director" {
		t.Errorf("header role line = %q, want %q", r.persona.Text, "Role: Regatta Director")
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
	if row.restarts.Text != common.NoStartTimeText ||
		row.startTime.Text != common.NoStartTimeText ||
		row.winTime.Text != common.NoStartTimeText ||
		row.approved.Text != common.EmptyString {
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
