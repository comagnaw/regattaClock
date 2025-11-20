package text

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func TestHeader1(t *testing.T) {
	testText := "Test Header 1"
	result := Header1(testText)

	if result == nil {
		t.Fatal("Header1 returned nil")
	}

	if result.Text != testText {
		t.Errorf("Expected text %q, got %q", testText, result.Text)
	}

	if result.Color != color.White {
		t.Errorf("Expected color to be White, got %v", result.Color)
	}

	if !result.TextStyle.Bold {
		t.Error("Expected text to be bold")
	}

	if result.TextStyle.Monospace {
		t.Error("Expected text to not be monospace")
	}

	if result.Alignment != fyne.TextAlignCenter {
		t.Errorf("Expected alignment to be Center, got %v", result.Alignment)
	}

	if result.TextSize != 48 {
		t.Errorf("Expected text size to be 48, got %f", result.TextSize)
	}
}

func TestHeader2(t *testing.T) {
	testText := "Test Header 2"
	result := Header2(testText)

	if result == nil {
		t.Fatal("Header2 returned nil")
	}

	if result.Text != testText {
		t.Errorf("Expected text %q, got %q", testText, result.Text)
	}

	if result.Color != color.White {
		t.Errorf("Expected color to be White, got %v", result.Color)
	}

	if !result.TextStyle.Bold {
		t.Error("Expected text to be bold")
	}

	if result.TextStyle.Monospace {
		t.Error("Expected text to not be monospace")
	}

	if result.Alignment != fyne.TextAlignCenter {
		t.Errorf("Expected alignment to be Center, got %v", result.Alignment)
	}

	if result.TextSize != 24 {
		t.Errorf("Expected text size to be 24, got %f", result.TextSize)
	}
}

func TestHeader3(t *testing.T) {
	testText := "Test Header 3"
	result := Header3(testText)

	if result == nil {
		t.Fatal("Header3 returned nil")
	}

	if result.Text != testText {
		t.Errorf("Expected text %q, got %q", testText, result.Text)
	}

	if result.Color != color.White {
		t.Errorf("Expected color to be White, got %v", result.Color)
	}

	if !result.TextStyle.Bold {
		t.Error("Expected text to be bold")
	}

	if result.TextStyle.Monospace {
		t.Error("Expected text to not be monospace")
	}

	if result.Alignment != fyne.TextAlignCenter {
		t.Errorf("Expected alignment to be Center, got %v", result.Alignment)
	}

	if result.TextSize != 20 {
		t.Errorf("Expected text size to be 20, got %f", result.TextSize)
	}
}

func TestCell(t *testing.T) {
	testText := "Test Cell"
	result := Cell(testText)

	if result == nil {
		t.Fatal("Cell returned nil")
	}

	if result.Text != testText {
		t.Errorf("Expected text %q, got %q", testText, result.Text)
	}

	if result.Color != color.Black {
		t.Errorf("Expected color to be Black, got %v", result.Color)
	}

	if !result.TextStyle.Bold {
		t.Error("Expected text to be bold")
	}

	if result.TextStyle.Monospace {
		t.Error("Expected text to not be monospace")
	}

	if result.Alignment != fyne.TextAlignCenter {
		t.Errorf("Expected alignment to be Center, got %v", result.Alignment)
	}

	if result.TextSize != 48 {
		t.Errorf("Expected text size to be 48, got %f", result.TextSize)
	}
}

func TestNewText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		color color.Color
		mono  bool
		bold  bool
		align fyne.TextAlign
		size  float32
	}{
		{
			name:  "standard text",
			text:  "Hello World",
			color: color.White,
			mono:  false,
			bold:  true,
			align: fyne.TextAlignCenter,
			size:  24,
		},
		{
			name:  "monospace text",
			text:  "Code Sample",
			color: color.Black,
			mono:  true,
			bold:  false,
			align: fyne.TextAlignLeading,
			size:  12,
		},
		{
			name:  "trailing aligned text",
			text:  "Right Aligned",
			color: color.RGBA{R: 255, G: 0, B: 0, A: 255},
			mono:  false,
			bold:  false,
			align: fyne.TextAlignTrailing,
			size:  16,
		},
		{
			name:  "empty string",
			text:  "",
			color: color.White,
			mono:  false,
			bold:  false,
			align: fyne.TextAlignCenter,
			size:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newText(tt.text, tt.color, tt.mono, tt.bold, tt.align, tt.size)

			if result == nil {
				t.Fatal("newText returned nil")
			}

			if result.Text != tt.text {
				t.Errorf("Expected text %q, got %q", tt.text, result.Text)
			}

			if result.Color != tt.color {
				t.Errorf("Expected color %v, got %v", tt.color, result.Color)
			}

			if result.TextStyle.Monospace != tt.mono {
				t.Errorf("Expected monospace to be %v, got %v", tt.mono, result.TextStyle.Monospace)
			}

			if result.TextStyle.Bold != tt.bold {
				t.Errorf("Expected bold to be %v, got %v", tt.bold, result.TextStyle.Bold)
			}

			if result.Alignment != tt.align {
				t.Errorf("Expected alignment to be %v, got %v", tt.align, result.Alignment)
			}

			if result.TextSize != tt.size {
				t.Errorf("Expected text size to be %f, got %f", tt.size, result.TextSize)
			}
		})
	}
}

func TestHeadersHaveDifferentSizes(t *testing.T) {
	h1 := Header1("Test")
	h2 := Header2("Test")
	h3 := Header3("Test")

	if h1.TextSize <= h2.TextSize {
		t.Errorf("Header1 size (%f) should be larger than Header2 size (%f)", h1.TextSize, h2.TextSize)
	}

	if h2.TextSize <= h3.TextSize {
		t.Errorf("Header2 size (%f) should be larger than Header3 size (%f)", h2.TextSize, h3.TextSize)
	}
}

func TestCellVsHeaderColor(t *testing.T) {
	cell := Cell("Test")
	header := Header1("Test")

	if cell.Color == header.Color {
		t.Error("Cell color should be different from Header color (Cell is Black, Header is White)")
	}

	if cell.Color != color.Black {
		t.Errorf("Cell color should be Black, got %v", cell.Color)
	}

	if header.Color != color.White {
		t.Errorf("Header color should be White, got %v", header.Color)
	}
}

func TestAllFunctionsReturnValidCanvasText(t *testing.T) {
	testText := "Test"

	functions := map[string]func(string) *canvas.Text{
		"Header1": Header1,
		"Header2": Header2,
		"Header3": Header3,
		"Cell":    Cell,
	}

	for name, fn := range functions {
		t.Run(name, func(t *testing.T) {
			result := fn(testText)

			if result == nil {
				t.Fatalf("%s returned nil", name)
			}

			// Verify it's a valid canvas.Text object with expected properties
			if result.Text == "" && testText != "" {
				t.Errorf("%s did not set text properly", name)
			}

			if result.Color == nil {
				t.Errorf("%s did not set color", name)
			}

			if result.TextSize <= 0 {
				t.Errorf("%s has invalid text size: %f", name, result.TextSize)
			}
		})
	}
}

func TestSpecialCharactersInText(t *testing.T) {
	specialStrings := []string{
		"Hello\nWorld",
		"Tab\tSeparated",
		"Unicode: 你好世界",
		"Emoji: 🎉🎊",
		"Special: @#$%^&*()",
		"",
	}

	for _, str := range specialStrings {
		t.Run("Header1_"+str, func(t *testing.T) {
			result := Header1(str)
			if result == nil {
				t.Fatal("Header1 returned nil")
			}
			if result.Text != str {
				t.Errorf("Expected text %q, got %q", str, result.Text)
			}
		})

		t.Run("Cell_"+str, func(t *testing.T) {
			result := Cell(str)
			if result == nil {
				t.Fatal("Cell returned nil")
			}
			if result.Text != str {
				t.Errorf("Expected text %q, got %q", str, result.Text)
			}
		})
	}
}
