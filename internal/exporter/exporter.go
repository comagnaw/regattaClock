package exporter

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strings"

	"github.com/comagnaw/regattaClock/internal/assets"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

const (
	width    = 1340
	height   = 927
	fontSize = 80
	dpi      = 72
)

func Export(regattaData reader.RegattaData) {
	
	for _, raceData := range regattaData.Races {

		if !raceData.HasBoats() {
			continue
		}

		fileName := fmt.Sprintf("race_%d_%s.png", raceData.RaceNumber, regattaData.Name)

		text := fmt.Sprintf("%s\n\n", raceData.RaceTitle())
		for laneNum, lane := range raceData.OrderedLanes() {
			if lane.SchoolName == common.EmptyString {
				continue
			}
			text += fmt.Sprintf("\nLane %d - %s\n", laneNum, lane.SchoolName)
		}	


		img := image.NewRGBA(image.Rect(0, 0, width, height))

		f, err := truetype.Parse(assets.VerdanaBoldFont)
		if err != nil {
			log.Fatalf("Error parsing font data: %v", err)
		}
	
		// 3. Configure the freetype context for TrueType drawing
		c := freetype.NewContext()
		c.SetDPI(dpi)
		c.SetFont(f)
		c.SetFontSize(float64(fontSize))
		c.SetClip(img.Bounds())
		c.SetDst(img)
		c.SetSrc(image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}))
	
		// 4. Draw the text using freetype, handling multiple lines.
		lines := strings.Split(text, "\n")
		// baseline starting pixel Y (10px margin from top of text)
		baseline := 10 + (fontSize * 3 / 4)
		// line spacing derived from the point size (approximate pixel height)
		lineSpacing := int(c.PointToFixed(float64(fontSize)) >> 6)
		if lineSpacing <= 0 {
			lineSpacing = fontSize
		}
		lineNum := 0
		titleBuffer := lineSpacing
		laneBuffer := lineSpacing / 3
		for _, ln := range lines {
			if ln == "" {
				continue
			}
			y := baseline + lineNum*lineSpacing
			if lineNum > 0 {
				y += titleBuffer + (lineNum-1)*laneBuffer
			}
			pt := freetype.Pt(10, y)
			if _, err = c.DrawString(ln, pt); err != nil {
				log.Fatalf("Error drawing text: %v", err)
			}
			lineNum++
		}
	
		// 5. Save the image to a PNG file
		outputFile, err := os.Create(fileName)
		if err != nil {
			log.Fatalf("Error creating output file: %v", err)
		}
		defer outputFile.Close()
	
		if err = png.Encode(outputFile, img); err != nil {
			log.Fatalf("Error encoding PNG: %v", err)
		}
	
		fmt.Printf("Image generated successfully as %s\n", fileName)







	}
	
	
}