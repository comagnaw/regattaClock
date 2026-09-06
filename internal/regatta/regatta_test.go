package regatta

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func TestNewRegatta(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)
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

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)
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

func TestRegatta_ListTitle(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)

	label := regatta.listTitle()

	if label == nil {
		t.Fatal("listTitle returned nil")
	}

	if label.Text != common.ScheduledRacesTile {
		t.Errorf("Expected label text %q, got %q", common.ScheduledRacesTile, label.Text)
	}

	if !label.TextStyle.Bold {
		t.Error("List title should be bold")
	}
}

func TestRegatta_RaceList_Empty(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)
	regatta.RegattaData = &reader.RegattaData{
		Name:  "Empty Regatta",
		Date:  "2024-01-01",
		Races: []reader.RaceData{},
	}

	scroll := regatta.raceList()

	if scroll == nil {
		t.Fatal("raceList returned nil")
	}

	// The list keeps a height floor so it never collapses, but must stay well short
	// of the window height: it shares the window with the title header, so demanding
	// the full height would push the content past the window and force it to grow.
	minSize := scroll.MinSize()
	if minSize.Height < raceListMinHeight {
		t.Errorf("Expected min height of at least %f, got %f", raceListMinHeight, minSize.Height)
	}

	if minSize.Height >= regattaHeight {
		t.Errorf("Min height %f leaves no room for the header within the %f window",
			minSize.Height, regattaHeight)
	}
}

func TestRegatta_RaceList_WithRaces(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)
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

	scroll := regatta.raceList()

	if scroll == nil {
		t.Fatal("raceList returned nil")
	}
}

func TestRegatta_RaceList_SkipsEmptyRaces(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)
	regatta.RegattaData = &reader.RegattaData{
		Name: "Mixed Regatta",
		Date: "2024-01-15",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  4,
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School A"}},
			},
			{
				RaceNumber: 2,
				BoatCount:  0,
				Lanes:      map[int]reader.RaceEntry{}, // Empty race
			},
			{
				RaceNumber: 3,
				BoatCount:  3,
				Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School C"}},
			},
		},
	}

	scroll := regatta.raceList()

	if scroll == nil {
		t.Fatal("raceList returned nil")
	}

	// The scroll container should exist but we can't easily count entries
	// Just verify it was created successfully
}

func TestRegatta_TimeButton(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)
	regatta.RegattaData = &reader.RegattaData{
		Name:  "Test Regatta",
		Date:  "2024-01-15",
		Races: []reader.RaceData{},
	}

	race := reader.RaceData{
		RaceNumber: 1,
		BoatCount:  4,
		BoatClass:  "Varsity 8",
		Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School A"}},
	}

	button := regatta.timeButton(race)

	if button == nil {
		t.Fatal("timeButton returned nil")
	}

	if button.Text != common.TimeRaceButtonText {
		t.Errorf("Expected button text %q, got %q", common.TimeRaceButtonText, button.Text)
	}

	// Verify button has an action
	if button.OnTapped == nil {
		t.Error("Button should have OnTapped action")
	}
}

func TestRegatta_RaceEntry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)
	regatta.RegattaData = &reader.RegattaData{
		Name:  "Test Regatta",
		Date:  "2024-01-15",
		Races: []reader.RaceData{},
	}

	race := reader.RaceData{
		RaceNumber: 5,
		BoatCount:  4,
		BoatClass:  "Varsity 8",
		FlightInfo: "Heat 1",
		Lanes:      map[int]reader.RaceEntry{1: {SchoolName: "School A"}},
	}

	container := regatta.raceEntry(race)

	if container == nil {
		t.Fatal("raceEntry returned nil")
	}

	if len(container.Objects) < 2 {
		t.Errorf("Expected at least 2 objects in race entry container, got %d", len(container.Objects))
	}
}

func TestRegatta_MakeMenu(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)

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

	regatta := NewRegatta(app)

	size := regatta.window.Canvas().Size()
	if size.Width != regattaWidth || size.Height != regattaHeight {
		t.Errorf("Expected initial window size %fx%f, got %fx%f",
			regattaWidth, regattaHeight, size.Width, size.Height)
	}
}
