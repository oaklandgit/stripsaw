package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Set STRIPSAW_DUMP=<dir> to write the rendered frames for a visual check.
func TestRender(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3000, 1000))
	frames := map[string]RenderOpts{
		"cols":         {Layout: LayoutCols},
		"cols-preview": {Layout: LayoutCols, ColSplits: [][]float64{{30, 66, 92}}, Numbers: true},
		"rows-preview": {Layout: LayoutRows, RowSplits: []float64{50}, ColSplits: [][]float64{{}, {}}, Numbers: true},
		"grid-preview": {Layout: LayoutGrid, RowSplits: []float64{50}, ColSplits: [][]float64{{40}, {25, 50, 75}}, Numbers: true},
		"grid-rows":    {Layout: LayoutGrid, RowSplits: []float64{50}, ColSplits: make([][]float64, 2), Numbers: true},
		"thumb":        {Plain: true, Layout: LayoutGrid, RowSplits: []float64{50}, ColSplits: [][]float64{{40}, {25, 75}}},
	}
	dump := os.Getenv("STRIPSAW_DUMP")
	for name, o := range frames {
		img := Render(src, 1600, 900, o)
		if b := img.Bounds(); b.Dx() > 1600 || b.Dy() > 900 {
			t.Errorf("%s: %v exceeds bounds", name, b)
		}
		if dump != "" {
			f, err := os.Create(filepath.Join(dump, name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			png.Encode(f, img)
			f.Close()
		}
	}
}
