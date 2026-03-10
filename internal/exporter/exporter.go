package exporter

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/comagnaw/regattaClock/internal/assets"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

const (
	width      = 1740
	height     = 780
	fontSize   = 80
	dpi        = 72
	leftMargin = 10
	topMargin  = 10
)

var fontColor = color.RGBA{R: 255, G: 255, B: 255, A: 255}

// ExportResult contains the summary of an export operation.
type ExportResult struct {
	Succeeded int
	Failed    int
	Errors    []error
}

// HasErrors returns true if any exports failed.
func (r ExportResult) HasErrors() bool {
	return r.Failed > 0
}

// Export generates PNG images for each race in the regatta data. Each race with
// boats produces a separate image file in the specified output directory. Races
// with no boats (BoatCount == 0) are skipped. Returns an ExportResult summarizing
// how many files succeeded and failed, along with any errors encountered.
func Export(regattaData reader.RegattaData, outputDir string) ExportResult {
	regattaName := strings.ReplaceAll(regattaData.Name, " ", "_")
	result := ExportResult{}

	for _, raceData := range regattaData.Races {
		if !raceData.HasBoats() {
			continue
		}

		fileName := buildFileName(outputDir, raceData.RaceNumber, regattaName)
		text := buildRaceText(raceData)
		img, err := renderImage(text)
		if err != nil {
			log.Printf("Error rendering race %d: %v", raceData.RaceNumber, err)
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("race %d: %w", raceData.RaceNumber, err))
			continue
		}

		if err := saveImage(img, fileName); err != nil {
			log.Printf("Error saving race %d to %s: %v", raceData.RaceNumber, fileName, err)
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("race %d: %w", raceData.RaceNumber, err))
			continue
		}

		result.Succeeded++
		fmt.Printf("Image generated successfully as %s\n", fileName)
	}

	return result
}

// buildFileName constructs the output file path for a race image. The race number
// is zero-padded to 2 digits (e.g., 1 -> "01", 10 -> "10") to ensure proper
// lexicographic sorting in file browsers.
func buildFileName(outputDir string, raceNumber int, regattaName string) string {
	return filepath.Join(outputDir, fmt.Sprintf("race_%02d_%s.png", raceNumber, regattaName))
}

// buildRaceText formats the race information into a multi-line string for display.
// The first line contains the race title (number, boat count, class, and flight info).
// Subsequent lines list each lane and school name in lane order. Lanes with empty
// school names are omitted from the output.
func buildRaceText(raceData reader.RaceData) string {
	var sb strings.Builder
	sb.WriteString(raceData.RaceTitle())
	sb.WriteString("\n")

	for laneNum, lane := range raceData.OrderedLanes() {
		if lane.SchoolName == common.EmptyString {
			continue
		}
		sb.WriteString(fmt.Sprintf("\nLane %d - %s", laneNum, lane.SchoolName))
	}

	return sb.String()
}

// renderImage creates a new RGBA image with the specified dimensions and renders
// the provided text onto it. The image has a transparent background with white text.
// Returns an error if the font fails to parse or text rendering fails.
func renderImage(text string) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	ctx, err := newDrawingContext(img)
	if err != nil {
		return nil, err
	}
	if err := drawText(ctx, text); err != nil {
		return nil, err
	}
	return img, nil
}

// newDrawingContext initializes a freetype context configured for text rendering.
// Uses the embedded Verdana Bold font with the configured fontColor. The context is
// clipped to the image bounds. Returns an error if the embedded font fails to parse,
// which should not occur under normal circumstances since the font is compiled in.
func newDrawingContext(img *image.RGBA) (*freetype.Context, error) {
	f, err := truetype.Parse(assets.VerdanaBoldFont)
	if err != nil {
		return nil, fmt.Errorf("parsing font data: %w", err)
	}

	c := freetype.NewContext()
	c.SetDPI(dpi)
	c.SetFont(f)
	c.SetFontSize(float64(fontSize))
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(image.NewUniform(fontColor))

	return c, nil
}

// drawText renders multi-line text onto the image using the provided freetype context.
// Empty lines are skipped rather than rendered as blank space. The first line (title)
// has extra buffer space below it, and subsequent lines (lanes) have smaller spacing
// between them. All spacing values scale proportionally with the font size.
// Returns an error if any text rendering operation fails.
func drawText(c *freetype.Context, text string) error {
	lines := strings.Split(text, "\n")

	baseline := topMargin + (fontSize * 3 / 4)
	lineSpacing := int(c.PointToFixed(float64(fontSize)) >> 6)
	if lineSpacing <= 0 {
		lineSpacing = fontSize
	}

	titleBuffer := lineSpacing
	laneBuffer := lineSpacing / 3

	lineNum := 0
	for _, ln := range lines {
		if ln == "" {
			continue
		}

		y := baseline + lineNum*lineSpacing
		if lineNum > 0 {
			y += titleBuffer + (lineNum-1)*laneBuffer
		}

		pt := freetype.Pt(leftMargin, y)
		if _, err := c.DrawString(ln, pt); err != nil {
			return fmt.Errorf("drawing text: %w", err)
		}
		lineNum++
	}
	return nil
}

// saveImage writes the RGBA image to disk as a PNG file. The output directory must
// exist; this function does not create parent directories. Returns an error if the
// file cannot be created or the PNG encoding fails.
func saveImage(img *image.RGBA, fileName string) error {
	outputFile, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outputFile.Close()

	if err = png.Encode(outputFile, img); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}
	return nil
}
