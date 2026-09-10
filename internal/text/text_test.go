package text

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

func TestBoldLabel(t *testing.T) {
	l := BoldLabel("Header")
	if l == nil {
		t.Fatal("BoldLabel returned nil")
	}
	if l.Text != "Header" {
		t.Errorf("Text = %q, want %q", l.Text, "Header")
	}
	if !l.TextStyle.Bold {
		t.Error("BoldLabel should be bold")
	}
}

func TestBoldLabelCenter(t *testing.T) {
	l := BoldLabelCenter("Header")
	if l == nil {
		t.Fatal("BoldLabelCenter returned nil")
	}
	if !l.TextStyle.Bold {
		t.Error("BoldLabelCenter should be bold")
	}
	if l.Alignment != fyne.TextAlignCenter {
		t.Errorf("Alignment = %v, want Center", l.Alignment)
	}
}

func TestTruncatingLabels(t *testing.T) {
	cases := []struct {
		name  string
		got   *widget.Label
		align fyne.TextAlign
	}{
		{"Truncating", Truncating("x"), fyne.TextAlignLeading},
		{"TruncatingTrailing", TruncatingTrailing("x"), fyne.TextAlignTrailing},
		{"TruncatingCenter", TruncatingCenter("x"), fyne.TextAlignCenter},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got == nil {
				t.Fatalf("%s returned nil", c.name)
			}
			if c.got.Text != "x" {
				t.Errorf("Text = %q, want %q", c.got.Text, "x")
			}
			if c.got.Truncation != fyne.TextTruncateEllipsis {
				t.Errorf("Truncation = %v, want ellipsis", c.got.Truncation)
			}
			if c.got.Alignment != c.align {
				t.Errorf("Alignment = %v, want %v", c.got.Alignment, c.align)
			}
		})
	}
}

func TestNote(t *testing.T) {
	l := Note("aside")
	if l == nil {
		t.Fatal("Note returned nil")
	}
	if l.Text != "aside" {
		t.Errorf("Text = %q, want %q", l.Text, "aside")
	}
	if !l.TextStyle.Italic {
		t.Error("Note should be italic")
	}
	if l.TextStyle.Bold {
		t.Error("Note should not be bold")
	}
	if l.Alignment != fyne.TextAlignLeading {
		t.Errorf("Alignment = %v, want Leading", l.Alignment)
	}
}

func TestWrapping(t *testing.T) {
	l := Wrapping("some long message")
	if l == nil {
		t.Fatal("Wrapping returned nil")
	}
	if l.Text != "some long message" {
		t.Errorf("Text = %q, want %q", l.Text, "some long message")
	}
	if l.Wrapping != fyne.TextWrapWord {
		t.Errorf("Wrapping = %v, want TextWrapWord", l.Wrapping)
	}
	if l.TextStyle.Bold || l.TextStyle.Italic {
		t.Error("Wrapping should not set TextStyle flags")
	}
}

func TestHeader1(t *testing.T) {
	testText := "Test Header 1"
	result := Header1(testText)

	if result == nil {
		t.Fatal("Header1 returned nil")
	}

	if result.Text != testText {
		t.Errorf("Expected text %q, got %q", testText, result.Text)
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

func TestBannerHeading(t *testing.T) {
	result := BannerHeading("Timing for Race 7")

	if result == nil {
		t.Fatal("BannerHeading returned nil")
	}
	if result.Text != "Timing for Race 7" {
		t.Errorf("Expected text %q, got %q", "Timing for Race 7", result.Text)
	}
	if !result.TextStyle.Bold {
		t.Error("Expected text to be bold")
	}
	if result.Alignment != fyne.TextAlignLeading {
		t.Errorf("Expected alignment to be Leading, got %v", result.Alignment)
	}
	if result.TextSize <= Header2("x").TextSize {
		t.Errorf("BannerHeading size (%f) should be larger than Header2 (%f)", result.TextSize, Header2("x").TextSize)
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
		mono  bool
		bold  bool
		align fyne.TextAlign
		size  float32
	}{
		{
			name:  "standard text",
			text:  "Hello World",
			mono:  false,
			bold:  true,
			align: fyne.TextAlignCenter,
			size:  24,
		},
		{
			name:  "monospace text",
			text:  "Code Sample",
			mono:  true,
			bold:  false,
			align: fyne.TextAlignLeading,
			size:  12,
		},
		{
			name:  "trailing aligned text",
			text:  "Right Aligned",
			mono:  false,
			bold:  false,
			align: fyne.TextAlignTrailing,
			size:  16,
		},
		{
			name:  "empty string",
			text:  "",
			mono:  false,
			bold:  false,
			align: fyne.TextAlignCenter,
			size:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newText(tt.text, tt.mono, tt.bold, tt.align, tt.size)

			if result == nil {
				t.Fatal("newText returned nil")
			}

			if result.Text != tt.text {
				t.Errorf("Expected text %q, got %q", tt.text, result.Text)
			}

			// if result.Color != tt.color {
			// 	t.Errorf("Expected color %v, got %v", tt.color, result.Color)
			// }

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
